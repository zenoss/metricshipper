package input

import (
	"encoding/json"
	"github.com/garyburd/redigo/redis"
	"github.com/iancmcc/metricd"
	"testing"
	"time"
)

var queue_name string = "metrics"

func newReader(t *testing.T) *RedisReader {
	r := &RedisReader{
		Incoming: make(chan metricd.Metric, 20),
		pool: &redis.Pool{
			MaxActive:   2,
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

	//n, err := redis.Int(c.Do("DBSIZE"))
	//if err != nil {
	//	return nil, err
	//}

	//if n != 0 {
	//	return nil, errors.New("database #9 is not empty, test can not continue")
	//}

	return testConn{c}, nil
}

func sendone(conn redis.Conn) {
	m := &metricd.Metric{}
	s, _ := json.Marshal(m)
	conn.Send("RPUSH", queue_name, s)
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
	reader.Incoming = make(chan metricd.Metric, 20)
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
}
