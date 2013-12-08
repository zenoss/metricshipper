package metricd

import (
	"encoding/json"
	"github.com/garyburd/redigo/redis"
	"testing"
	"time"
)

var queue_name string = "metrics"

func newReader(t *testing.T) *RedisReader {
	r := &RedisReader{
		Incoming: make(chan Metric, 20),
		pool: &redis.Pool{
			MaxActive:   3,
			IdleTimeout: 10,
			Dial:        dial,
		},
		concurrency:  1,
		batch_size:   10,
		queue_name:   queue_name,
		control_name: "metrics-control",
	}
	return r
}

type testConn struct {
	redis.Conn
}

func (t testConn) Close() error {
	_, err := t.Conn.Do("SELECT", "9")
	if err != nil {
		return nil
	}
	_, err = t.Conn.Do("FLUSHDB")
	if err != nil {
		return err
	}
	return t.Conn.Close()
}

func dial() (redis.Conn, error) {
	c, err := redis.DialTimeout("tcp", ":6379", 0, 1*time.Second, 1*time.Second)
	if err != nil {
		return nil, err
	}

	_, err = c.Do("SELECT", "9")
	if err != nil {
		return nil, err
	}

	return testConn{c}, nil
}

func sendone(conn redis.Conn) {
	m := &Metric{}
	s, _ := json.Marshal(m)
	conn.Send("RPUSH", queue_name, s)
}

func TestParseRedisUri(t *testing.T) {
	config, err := ParseRedisUri("redis://localhost:6379/0/channel")
	if err != nil {
		t.Error("Unable to parse valid URL")
	}
	if config.Dialect != "redis" {
		t.Error("Unexpected scheme")
	}
	if config.Host != "localhost" {
		t.Error("Unexpected host")
	}
	if config.Port != 6379 {
		t.Error("Unexpected port")
	}
	if config.Database != "0" {
		t.Error("Unexpected db")
	}
	if config.Channel != "channel" {
		t.Error("Unexpected channel")
	}
}

func TestReadBatch(t *testing.T) {
	reader := newReader(t)
	reader.batch_size = 2
	conn := reader.pool.Get()
	defer conn.Close()
	defer conn.Do("DEL", queue_name)
	for i := 0; i < 3; i++ {
		sendone(conn)
	}
	reader.ReadBatch(conn)
	close(reader.Incoming)
	seen := make([]interface{}, 0)
	for m := range reader.Incoming {
		seen = append(seen, m)
	}
	if len(seen) != 2 {
		t.Error("Did not read the correct batch size")
	}
	reader.Incoming = make(chan Metric, 20)
	reader.ReadBatch(conn)
	close(reader.Incoming)
	for m := range reader.Incoming {
		seen = append(seen, m)
	}
	if len(seen) != 3 {
		t.Error("Did not read the correct batch size")
	}

}

func TestDrain(t *testing.T) {
	reader := newReader(t)
	reader.batch_size = 2
	conn := reader.pool.Get()
	defer conn.Close()
	defer conn.Do("DEL", queue_name)
	conn.Do("DEL", queue_name)
	for i := 0; i < 10; i++ {
		sendone(conn)
	}
	conn.Flush()
	reader.Drain()
	close(reader.Incoming)
	seen := make([]interface{}, 0)
	for m := range reader.Incoming {
		seen = append(seen, m)
	}
	if len(seen) != 10 {
		t.Log("Saw", len(seen), "metrics")
		t.Error("Did not read the correct batch size")
	}
}

func TestSubscribe(t *testing.T) {
	// Create and subscribe a reader
	reader := newReader(t)
	go reader.Subscribe()

	// Add 10 messages
	conn := reader.pool.Get()
	defer conn.Close()
	defer conn.Do("DEL", queue_name)
	conn.Do("DEL", queue_name)
	for i := 0; i < 10; i++ {
		sendone(conn)
	}
	conn.Flush()
	llen, _ := redis.Int(conn.Do("LLEN", queue_name))
	if llen != 10 {
		t.Error("Messages did not make it to redis")
	}

	// Now send a control message
	conn.Send("RPUSH", queue_name+"-control", 1)
	// Give subscriber some cycles to read
	time.Sleep(5 * time.Millisecond)

	// Check the length now, should have been read
	llen, _ = redis.Int(conn.Do("LLEN", queue_name))
	if llen != 0 {
		t.Error("Subscriber didn't hear control message")
	}
}
