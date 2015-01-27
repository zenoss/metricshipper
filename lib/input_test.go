package metricshipper

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/garyburd/redigo/redis"
	metrics "github.com/rcrowley/go-metrics"
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
		concurrency:   1,
		batch_size:    10,
		queue_name:    queue_name,
		IncomingMeter: metrics.NewMeter(),
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

func sendone(metric string, conn redis.Conn) error {
	m := &Metric{Metric: metric}
	s, _ := json.Marshal(m)
	return conn.Send("RPUSH", queue_name, s)
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
	mLen := 3
	mBatch := 2
	reader := newReader(t)
	reader.batch_size = mBatch
	conn := reader.pool.Get()
	defer conn.Close()
	defer conn.Do("DEL", queue_name)
	for i := 0; i < mLen; i++ {
		sendone(strconv.Itoa(i), conn)
	}
	conn.Flush()
	reader.ReadBatch(&conn)
	close(reader.Incoming)
	seen := make([]interface{}, 0)
	for m := range reader.Incoming {
		seen = append(seen, m)
	}
	if len(seen) != mBatch {
		t.Errorf("Did not read the correct batch size, expected 2 got %s", len(seen))
	}

	metrics, err := readMetrics(conn)
	if err != nil {
		t.Errorf("error reading metrics %v", err)
	}
	if len(metrics) != mLen-mBatch {
		t.Errorf("expected %v got %v", mLen-mBatch, len(metrics))
	}
	reader.Incoming = make(chan Metric, 20)
	reader.ReadBatch(&conn)
	close(reader.Incoming)
	for m := range reader.Incoming {
		seen = append(seen, m)
	}
	if len(seen) != 3 {
		t.Errorf("Did not read the correct batch size, expected 3 got %s", len(seen))
	}

}

func readMetrics(conn redis.Conn) ([]*Metric, error) {
	var metrics []*Metric
	conn.Send("LRANGE", queue_name, 0, 100) // read everything
	conn.Flush()
	json, err := redis.Strings(conn.Receive())
	if err != nil {
		return metrics, err
	}
	for _, x := range json {
		met, err := MetricFromJSON([]byte(x))
		if err != nil {
			return metrics, err
		}
		metrics = append(metrics, met)
	}

	return metrics, nil

}

func TestHandleNilMetric(t *testing.T) {
	reader := newReader(t)
	reader.batch_size = 3
	conn := reader.pool.Get()
	defer conn.Close()
	defer conn.Do("DEL", queue_name)

	// Send another batch of valid
	sendone("1", conn)
	sendone("2", conn)
	sendone("3", conn)

	// Send a nil
	conn.Send("RPUSH", queue_name, nil)
	// Send something valid
	sendone("4", conn)
	// Send another nil
	conn.Send("RPUSH", queue_name, nil)

	reader.ReadBatch(&conn)
	close(reader.Incoming)

	seen := make([]interface{}, 0)
	for m := range reader.Incoming {
		seen = append(seen, m)
	}
	if len(seen) != 1 {
		t.Errorf("Did not handle a nil metric correctly, expected 1, got %v : %#v", len(seen), seen)
	}

	reader.Incoming = make(chan Metric, 20)
	reader.ReadBatch(&conn)
	close(reader.Incoming)
	seen = make([]interface{}, 0)
	for m := range reader.Incoming {
		seen = append(seen, m)
	}
	if len(seen) != 3 {
		t.Error("Did not continue reading properly")
	}
}

func TestInvalidMetric(t *testing.T) {
	reader := newReader(t)
	reader.batch_size = 3
	conn := reader.pool.Get()
	defer conn.Close()
	defer conn.Do("DEL", queue_name)

	m := &Metric{}
	s, _ := json.Marshal(m)
	s = []byte(string(s) + "INVALID_JSON")

	// Send another batch of valid
	sendone("1", conn)
	sendone("2", conn)
	sendone("3", conn)

	// Send invalid JSON
	conn.Send("RPUSH", queue_name, s)
	// Send something valid
	sendone("4", conn)
	// Send more invalid JSON
	conn.Send("RPUSH", queue_name, s)

	reader.ReadBatch(&conn)
	close(reader.Incoming)

	seen := make([]interface{}, 0)
	for m := range reader.Incoming {
		seen = append(seen, m)
	}
	if len(seen) != 1 {
		t.Error("Did not handle invalid JSON correctly")
	}

	reader.Incoming = make(chan Metric, 20)
	reader.ReadBatch(&conn)
	close(reader.Incoming)
	seen = make([]interface{}, 0)
	for m := range reader.Incoming {
		seen = append(seen, m)
	}
	if len(seen) != 3 {
		t.Error("Did not continue reading properly")
	}
}

func TestDrain(t *testing.T) {
	reader := newReader(t)
	reader.batch_size = 2
	conn := reader.pool.Get()
	defer conn.Close()
	defer conn.Do("DEL", queue_name)
	if _, err := conn.Do("DEL", queue_name); err != nil {
		t.Fatalf("could not call DEL: %s", err)
	}
	for i := 0; i < 10; i++ {
		if err := sendone(strconv.Itoa(i), conn); err != nil {
			t.Fatalf("could send item: %s", err)
		}
	}
	if err := conn.Flush(); err != nil {
		t.Fatalf("Could not flush: %s", err)
	}
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
	if _, err := conn.Do("DEL", queue_name); err != nil {
		t.Fatalf("Could not call DEL: %s", err)
	}
	for i := 0; i < 10; i++ {
		if err := sendone(strconv.Itoa(i), conn); err != nil {
			t.Fatalf("could not send item: %s", err)
		}
	}
	if err := conn.Flush(); err != nil {
		t.Fatalf("could not flush: %s", err)
	}
	llen, err := redis.Int(conn.Do("LLEN", queue_name))
	if err != nil {
		t.Fatalf("error reading LLEN: %s", err)
	}
	if llen != 10 {
		t.Error("Messages did not make it to redis")
	}

	// Give subscriber some cycles to read
	time.Sleep(1 * time.Second)

	// Check the length now, should have been read
	llen, err = redis.Int(conn.Do("LLEN", queue_name))
	if err != nil {
		t.Fatalf("error reading LLEN: %s", err)
	}
	if llen != 0 {
		t.Error("Subscriber didn't hear control message")
	}
}
