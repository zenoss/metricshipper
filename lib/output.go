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

func (w *WebsocketPublisher) ReleaseConn(conn *websocket.Conn) {
	w.conn <- conn
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

func (w *WebsocketPublisher) DoBatch() {
	for {
		buf := make([]Metric, 0)
		batch := &MetricBatch{
			Metrics: buf,
		}
	Retry:
		for {
			conn := w.GetConn()
			timer := time.After(time.Duration(w.batch_timeout) * time.Second)
			remaining := w.batch_size - len(batch.Metrics)
		Batch:
			for i := 0; i < remaining; i++ {
				select {
				case <-timer:
					break Batch
				case m := <-w.Outgoing:
					buf = append(buf, m)
				}
			}
			if len(buf) > 0 {
				batch.Metrics = buf
				err := websocket.JSON.Send(conn, batch)
				if err != nil {
					// Dead connection; don't put it back, add another
					glog.Infoln("stuff")
					conn.Close()
					w.AddConn()
				} else {
					w.ReleaseConn(conn)
					break Retry
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
