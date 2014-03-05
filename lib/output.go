package metricshipper

import (
	"code.google.com/p/go.net/websocket"
	"encoding/base64"
	"github.com/zenoss/glog"
	"strings"
	"time"
)

var origin string = "http://localhost"

type WebsocketPublisher struct {
	config                   *websocket.Config
	concurrency              int
	conn                     chan *websocket.Conn
	batch_size               int
	batch_timeout            float64
	Outgoing                 chan Metric
	retry_connection         int
	retry_connection_timeout time.Duration //seconds
}

func NewWebsocketPublisher(uri string, concurrency int, buffer_size int,
	batch_size int, batch_timeout float64, retry_connection int,
	retry_connection_timeout time.Duration, username string,
	password string) (publisher *WebsocketPublisher, err error) {

	config, err := websocket.NewConfig(uri, origin)
	if err != nil {
		return nil, err
	}

	data := []byte(username + ":" + password)
	str := base64.StdEncoding.EncodeToString(data)
	config.Header.Add("Authorization", "basic "+str)

	return &WebsocketPublisher{
		config:                   config,
		concurrency:              concurrency,
		conn:                     make(chan *websocket.Conn, concurrency),
		batch_size:               batch_size,
		batch_timeout:            batch_timeout,
		Outgoing:                 make(chan Metric, buffer_size),
		retry_connection:         retry_connection,
		retry_connection_timeout: retry_connection_timeout,
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

func (w *WebsocketPublisher) ReleaseConn(conn *websocket.Conn, dead *bool) {
	glog.V(2).Infof("enter ReleaseConn(), conn=%v, dead=%t", conn.Config().Location, *dead)
	defer glog.V(2).Infof(" exit ReleaseConn(), conn=%v, dead=%t", conn.Config().Location, *dead)

	if *dead {
		conn.Close()
		w.AddConn()
	} else {
		w.conn <- conn
	}
}

func (w *WebsocketPublisher) AddConn() (err error) {
	attempts := 0
	for {
		if w.retry_connection > 0 && attempts >= w.retry_connection {
			break
		}
		if conn, dialerr := websocket.DialConfig(w.config); dialerr == nil {
			glog.Info("Made connection to consumer")
			w.conn <- conn
			break
		} else {
			err = dialerr
			glog.Errorf("Error connecting to (%+v), attempt %d/%d: %s", w.config.Location, attempts, w.retry_connection, err)
			time.Sleep(w.retry_connection_timeout * time.Second)
		}
		attempts += 1
	}

	if err != nil {
		glog.Error(err)
		return err
	}
	return nil
}

func (w *WebsocketPublisher) getBatch() (int, *MetricBatch) {
	glog.V(3).Infof("enter getBatch()")
	buf := make([]Metric, 0)
	batch := &MetricBatch{
		Metrics: buf,
	}
	defer glog.V(3).Infof("exit getBatch(), len(buf)=%d", len(buf))

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
	var dead *bool = new(bool)
	*dead = false
	conn := w.GetConn()
	glog.V(3).Infof("enter sendBatch(), conn=%s, len(batch)=%d", conn.Config().Location, len(batch.Metrics))
	defer glog.V(3).Infof("exit sendBatch(), dead=%t, num=%d", *dead, num)
	defer w.ReleaseConn(conn, dead)

	err := websocket.JSON.Send(conn, batch)
	if err != nil {
		*dead = true
		return num, err
	}

	num = len(batch.Metrics)
	*dead, err = w.readResponse(conn)

	return num, err
}

//read everything in the response buffer
func (w *WebsocketPublisher) readResponse(conn *websocket.Conn) (bool, error) {
	var err error
	dead := false

	for {
		n := 0
		deadline := time.Now().Add(time.Microsecond)
		err = conn.SetReadDeadline(deadline)

		msg := make([]byte, 1024)
		if n, err = conn.Read(msg); err != nil && !strings.HasSuffix(err.Error(), "i/o timeout") {
			dead = true
			break
		}

		err = nil
		msg = msg[0:n]
		if n == 0 {
			break
		}

		glog.V(2).Infof("Server responded with message: %s", string(msg))
	}

	return dead, err
}

func (w *WebsocketPublisher) DoBatch() {
	for {
		// Retry loop
		num, batch := w.getBatch()
		if num > 0 {
			for {
				sent, err := w.sendBatch(batch)
				if err == nil {
					glog.V(2).Infof("Sent %d metrics to the consumer.", sent)
					break
				} else {
					glog.Errorf("Failed sending %d metrics to the consumer: %s", num, err)
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
