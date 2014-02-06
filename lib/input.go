package metricshipper

import (
	"fmt"
	"github.com/garyburd/redigo/redis"
	"github.com/zenoss/glog"
	"net/url"
	"strconv"
	"strings"
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

func (c *RedisConnectionConfig) ControlChannel() string {
	return c.Channel + "-control"
}

func parseRedisUri(uri string) (config *RedisConnectionConfig, err error) {
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
func dialFunc(config *RedisConnectionConfig) func() (redis.Conn, error) {
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
	Incoming     chan Metric
	pool         *redis.Pool
	concurrency  int
	batch_size   int
	queue_name   string
	control_name string
}

// Read a batch of metrics
func (r *RedisReader) ReadBatch(conn redis.Conn) int {
	var count int
	var rangeresult []string
	glog.V(2).Infof("enter RedisReader.ReadBatch( conn=%s)", conn)
	defer glog.V(2).Infof("exit RedisReader.ReadBatch( conn=%s) count=%d", conn, count)

	// read redis values - Read in a chunk of metrics up to the batch size
	glog.V(2).Infof("RedisReader.ReadBatch( ) -- Sending Commands")
	var send_err error
	if send_err = conn.Send("MULTI"); send_err != nil {
		glog.Errorf("Error sending command, multi: %s", send_err)
	}

	if send_err = conn.Send("LRANGE", r.queue_name, 0, r.batch_size-1); send_err != nil {
		glog.Errorf("Error sending command, lrange: %s", send_err)
	}

	if send_err = conn.Send("LTRIM", r.queue_name, r.batch_size, -1); send_err != nil {
		glog.Errorf("Error sending command, ltrim: %s", send_err)
	}

	//read redis values
	glog.V(2).Infof("RedisReader.ReadBatch( ) -- Reading Values")
	values, err := redis.Values(conn.Do("EXEC"))
	if err != nil {
		glog.Errorf("Error retrieving metric values: %s", err)
	}

	//scan redis values
	glog.V(2).Infof("RedisReader.ReadBatch( ) -- Scanning Values")
	if _, err := redis.Scan(values, &rangeresult); err != nil {
		glog.Errorf("Error scanning metric values: %s", err)
	}

	// Else, deserialize each metric and shove it down the channel
	glog.V(2).Infof("RedisReader.ReadBatch( ) -- Parsing Values")
	for _, m := range rangeresult {
		met, err := MetricFromJSON([]byte(m))
		if err != nil {
			glog.Errorf("Invalid metric json: %s", err)
		} else {
			r.Incoming <- *met
		}
	}

	count = len(rangeresult)
	return count
}

// Drain the redis queue into the out channel until there's nothing left.
func (r *RedisReader) Drain() {
	conn := r.pool.Get()
	glog.V(2).Infof("enter RedisReader.Drain()")
	defer glog.V(2).Infof("exit RedisReader.Drain()")
	defer conn.Close()
	for {
		count := r.ReadBatch(conn)
		// If no metrics were returned, this goroutine's job is done
		if count == 0 {
			break
		}
	}
}

// Start listening to the control channel
func (r *RedisReader) Subscribe() (err error) {
	for i := 0; i < r.concurrency; i++ {
		go r.Drain()
	}
	conn := r.pool.Get()
	defer conn.Close()
	psc := redis.PubSubConn{
		Conn: conn,
	}
	err = psc.Subscribe(r.control_name)
	if err != nil {
		glog.Error("Unable to subscribe to metrics channel")
	}
	defer psc.Unsubscribe()
	for {
		m := psc.Receive()
		switch m.(type) {
		case redis.Message:
			// Up the concurrency
			if r.pool.ActiveCount() < r.concurrency+1 {
				go r.Drain()
			}
			break
		case redis.Subscription:
			continue
		case redis.PMessage:
			continue
		case error:
			return
		}
	}
	return nil
}

func NewRedisReader(uri string, batch_size int, buffer_size int,
	concurrency int) (reader *RedisReader, err error) {
	config, err := parseRedisUri(uri)
	if err != nil {
		return nil, err
	}
	glog.Infoln("Connecting to redis server", config.Server())
	glog.Infoln("Metrics database:", config.Database)
	glog.Infoln("Metrics queue name:", config.Channel)
	glog.Infoln("Concurrency:", concurrency)
	reader = &RedisReader{
		Incoming: make(chan Metric, buffer_size),
		pool: &redis.Pool{
			MaxActive:   concurrency + 2,
			IdleTimeout: 240 * time.Second, // TODO: Configurable?
			Dial:        dialFunc(config),
		},
		concurrency:  concurrency,
		batch_size:   batch_size,
		queue_name:   config.Channel,
		control_name: config.ControlChannel(),
	}
	return reader, nil
}
