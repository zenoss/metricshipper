package metricshipper

import (
	"code.google.com/p/go.net/websocket"
	"encoding/base64"
	"github.com/zenoss/glog"
	"time"
)

var origin string = "http://localhost"

type WebsocketPublisher struct {
	config        *websocket.Config
	concurrency   int
	conn          chan *websocket.Conn
	batch_size    int
	batch_timeout float64
	Outgoing      chan Metric
}

func NewWebsocketPublisher(uri string, concurrency int, buffer_size int,
	batch_size int, batch_timeout float64, username string,
	password string) (publisher *WebsocketPublisher, err error) {

	config, err := websocket.NewConfig(uri, origin)
	if err != nil {
		return nil, err
	}

	data := []byte(username + ":" + password)
	str := base64.StdEncoding.EncodeToString(data)
	config.Header.Add("Authorization", "basic "+str)

	return &WebsocketPublisher{
		config:        config,
		concurrency:   concurrency,
		conn:          make(chan *websocket.Conn, concurrency),
		batch_size:    batch_size,
		batch_timeout: batch_timeout,
		Outgoing:      make(chan Metric, buffer_size),
	}, nil
}

func (w *WebsocketPublisher) InitConns() (err error) {
	for x := 0; x < w.concurrency; x++ {
		err := w.AddConn()
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *WebsocketPublisher) GetConn() *websocket.Conn {
	c := <-w.conn
	return c
}

func (w *WebsocketPublisher) ReleaseConn(conn *websocket.Conn, dead bool) {
	if dead {
		w.AddConn()
	} else {
		w.conn <- conn
	}
}

func (w *WebsocketPublisher) AddConn() (err error) {
	conn, err := websocket.DialConfig(w.config)
	if err != nil {
		glog.Error(err)
		return err
	}
	w.conn <- conn
	glog.Info("Made connection to consumer")
	return nil
}

func (w *WebsocketPublisher) getBatch() (int, *MetricBatch) {
	buf := make([]Metric, 0)
	batch := &MetricBatch{
		Metrics: buf,
	}
	remaining := w.batch_size - len(buf)
	timer := time.After(time.Duration(w.batch_timeout) * time.Second)
	for i := 0; i < remaining; i++ {
		select {
		case <-timer:
			i = remaining // Break out of the loop
		case m := <-w.Outgoing:
			buf = append(buf, m)
		}
	}
	batch.Metrics = buf
	return len(buf), batch
}

func (w *WebsocketPublisher) sendBatch(batch *MetricBatch) (int, error) {
	var num int
	dead := false
	conn := w.GetConn()
	defer w.ReleaseConn(conn, dead)
	err := websocket.JSON.Send(conn, batch)
	if err != nil {
		dead = true
		return num, err
	}
	num = len(batch.Metrics)
	return num, nil
}

func (w *WebsocketPublisher) DoBatch() {
	for {
		// Retry loop
		num, batch := w.getBatch()
		if num > 0 {
			for {
				sent, err := w.sendBatch(batch)
				if err == nil {
					glog.Infof("Sent %d metrics to the consumer.", sent)
					break
				}
			}
		}
	}
}

func (w *WebsocketPublisher) Start() (err error) {
	// Start up the forwarders. They'll all block on GetConn()
	for i := 0; i < w.concurrency; i++ {
		go w.DoBatch()
	}
	// Start trying to connect
	return w.InitConns()
}
