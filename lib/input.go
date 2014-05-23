package metricshipper

import (
	"fmt"
	"github.com/garyburd/redigo/redis"
	"github.com/rcrowley/go-metrics"
	"github.com/zenoss/glog"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type RedisConnectionConfig struct {
	Dialect  string
	Host     string
	Port     int
	Database string
	Channel  string
}

func (c *RedisConnectionConfig) Server() string {
	return c.Host + fmt.Sprintf(":%d", c.Port)
}

func ParseRedisUri(uri string) (config *RedisConnectionConfig, err error) {
	config = &RedisConnectionConfig{}
	parsed, err := url.Parse(uri)
	if err != nil {
		return config, err
	}
	config.Dialect = parsed.Scheme
	if strings.Contains(parsed.Host, ":") {
		parts := strings.SplitN(parsed.Host, ":", 2)
		config.Host = parts[0]
		if len(parts) > 1 {
			config.Port, _ = strconv.Atoi(parts[1])
		}
	}
	glog.Infoln("Path:", parsed.Path)
	segments := strings.Split(strings.TrimLeft(parsed.Path, "/"), "/")
	if len(segments) > 0 {
		config.Database = segments[0]
	}
	if len(segments) > 1 {
		config.Channel = segments[1]
	}
	return config, nil
}

// Connection initialization func
func DialFunc(config *RedisConnectionConfig) func() (redis.Conn, error) {
	return func() (redis.Conn, error) {
		c, err := redis.Dial("tcp", config.Server())
		if err != nil {
			glog.Error("Unable to connect to Redis")
			return nil, err
		}
		_, err = c.Do("SELECT", config.Database)
		if err != nil {
			glog.Error("Unable to select database")
			return nil, err
		}
		return c, nil
	}
}

// Reads metrics from redis
type RedisReader struct {
	Incoming      chan Metric
	pool          *redis.Pool
	concurrency   int
	batch_size    int
	queue_name    string
	IncomingMeter metrics.Meter // no need to lock since metrics.Meter already does that
}

// Read a batch of metrics
func (r *RedisReader) ReadBatch(conn *redis.Conn) int {
	var rangeresult []string
	glog.V(2).Infof("enter RedisReader.ReadBatch( conn=%v)", &(*conn))

	// read redis values - Read in a chunk of metrics up to the batch size
	glog.V(2).Infof("RedisReader.ReadBatch( ) -- Sending Commands")
	var send_err error
	if send_err = (*conn).Send("MULTI"); send_err != nil {
		glog.Errorf("Error sending command, multi: %s", send_err)
		return -1
	}

	if send_err = (*conn).Send("LRANGE", r.queue_name, 0, r.batch_size-1); send_err != nil {
		glog.Errorf("Error sending command, lrange: %s", send_err)
		return -1
	}

	if send_err = (*conn).Send("LTRIM", r.queue_name, r.batch_size, -1); send_err != nil {
		glog.Errorf("Error sending command, ltrim: %s", send_err)
		return -1
	}

	//read redis values
	glog.V(2).Infof("RedisReader.ReadBatch( ) -- Reading Values")
	values, err := redis.Values((*conn).Do("EXEC"))
	if err != nil {
		glog.Errorf("Error retrieving metric values: %s", err)
		return -1
	}

	//scan redis values
	glog.V(2).Infof("RedisReader.ReadBatch( ) -- Scanning Values")
	if _, err := redis.Scan(values, &rangeresult); err != nil {
		glog.Errorf("Error scanning metric values: %s", err)
		return -1
	}

	// Else, deserialize each metric and shove it down the channel
	glog.V(2).Infof("RedisReader.ReadBatch( ) -- Parsing Values")
	for _, m := range rangeresult {
		met, err := MetricFromJSON([]byte(m))
		if err != nil {
			glog.Errorf("Invalid metric json: %s", err)
		} else {
			r.Incoming <- *met
			glog.V(3).Infof("METRIC INC %+v", *met)
		}
	}

	// update meter with number of metrics read
	r.IncomingMeter.Mark(int64(len(rangeresult)))

	glog.V(2).Infof("exit RedisReader.ReadBatch( conn=%v) count=%d", &(*conn), len(rangeresult))
	return len(rangeresult)
}

// Drain the redis queue into the out channel until there's nothing left.
func (r *RedisReader) Drain() {
	glog.V(2).Infof("enter RedisReader.Drain()")
	defer glog.V(2).Infof("exit RedisReader.Drain()")
	for {
		done := false

		//loop over the same connection until an error occurs or no metrics are available
		func() {
			conn := r.pool.Get()
			defer conn.Close()
			for {
				count := r.ReadBatch(&conn)
				// If no metrics were returned, this goroutine's job is done
				if count == 0 {
					done = true
					break
				}

				// there was an error pulling data, create a new connection
				if count < 0 {
					break
				}
			}
		}()

		// draining complete
		if done {
			break
		}
	}
}

func NewRedisReader(uri string, batch_size int, buffer_size int,
	concurrency int) (reader *RedisReader, err error) {
	config, err := ParseRedisUri(uri)
	if err != nil {
		return nil, err
	}

	incomingMeter := metrics.NewMeter()
	metrics.Register("incomingMeter", incomingMeter)

	glog.Infoln("Connecting to redis server", config.Server())
	glog.Infoln("Metrics database:", config.Database)
	glog.Infoln("Metrics queue name:", config.Channel)
	glog.Infoln("Concurrency:", concurrency)
	reader = &RedisReader{
		Incoming: make(chan Metric, buffer_size),
		pool: &redis.Pool{
			MaxActive:   concurrency + 2,
			IdleTimeout: 240 * time.Second, // TODO: Configurable?
			Dial:        DialFunc(config),
		},
		concurrency:   concurrency,
		batch_size:    batch_size,
		queue_name:    config.Channel,
		IncomingMeter: incomingMeter,
	}
	return reader, nil
}

// terminate subscription
var terminate int = 0

// retry polling
var retry int = 1

// data's available, start draining
var drain int = 2

// Start listening for metrics by polling the metric queue
func (r *RedisReader) Subscribe() (err error) {
	//spawn go routines and wait for them to stop
	var complete sync.WaitGroup
	for i := 0; i < r.concurrency; i += 1 {
		complete.Add(1)
		go func() {
			defer complete.Done()
			//poll for data, then drain
			for {
				status, _err := r.poll()
				if status == terminate {
					err = _err
					break
				} else if status == drain {
					r.Drain()
				}
			}
		}()
	}
	complete.Wait()
	return nil
}

// poll the metrics list
func (r *RedisReader) poll() (status int, err error) {
	conn := r.pool.Get()
	defer conn.Close()

	// loop until data exists
	for {
		length, err := redis.Int(conn.Do("LLEN", r.queue_name))
		if err != nil {
			//TODO terminate logic
			glog.Errorf("Error \"LLEN {}\": {}", r.queue_name, err)
			return retry, err
		}
		if length > 0 {
			return drain, nil
		}

		time.Sleep(1 * time.Second)
	}
}
