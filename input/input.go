package input

import (
	"fmt"
	"github.com/garyburd/redigo/redis"
	"github.com/iancmcc/metricd"
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
func dialFunc(config *RedisConnectionConfig) func() (redis.Conn, error) {
	return func() (redis.Conn, error) {
		c, err := redis.Dial("tcp", config.Server())
		if err != nil {
			glog.Fatal("Unable to connect to Redis")
			return nil, err
		}
		glog.Infoln("Dialed connection to redis")
		_, err = c.Do("SELECT", config.Database)
		if err != nil {
			glog.Fatal("Unable to select database")
			return nil, err
		}
		glog.Infoln("Using database", config.Database)
		return c, nil
	}
}

// Reads metrics from redis
type RedisReader struct {
	Incoming     chan metricd.Metric
	pool         *redis.Pool
	concurrency  int
	batch_size   int
	queue_name   string
	control_name string
}

// Drain the redis queue into the out channel until there's nothing left.
func (r *RedisReader) Drain() {
	conn := r.pool.Get()
	defer conn.Close()
	for {
		var rangeresult []string

		// Read in a chunk of metrics up to the batch size
		conn.Send("MULTI")
		conn.Send("LRANGE", r.queue_name, 0, r.batch_size-1)
		conn.Send("LTRIM", r.queue_name, r.batch_size, -1)
		values, err := redis.Values(conn.Do("EXEC"))
		if err != nil {
			glog.Fatal("Error retrieving redis values")
		}
		if _, err := redis.Scan(values, &rangeresult); err != nil {
			glog.Infoln(err)
			glog.Fatal("Error scanning redis values")
		}
		// If no metrics were returned, this goroutine's job is done
		if len(rangeresult) == 0 {
			glog.Info("Stopping reading from redis, nothing left")
			break
		}

		// Else, deserialize each metric and shove it down the channel
		for _, m := range rangeresult {
			met, err := metricd.MetricFromJSON([]byte(m))
			if err != nil {
				glog.Fatal("Invalid metric")
			} else {
				r.Incoming <- *met
			}
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
		glog.Fatal("Unable to subscribe to redis")
	}
	glog.Info("Subscribed to redis")
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
	config, err := ParseRedisUri(uri)
	if err != nil {
		return nil, err
	}
	glog.Infoln("Connecting to redis server", config.Server())
	glog.Infoln("Metrics database:", config.Database)
	glog.Infoln("Metrics queue name:", config.Channel)
	glog.Infoln("Concurrency:", concurrency)
	reader = &RedisReader{
		Incoming: make(chan metricd.Metric, buffer_size),
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
