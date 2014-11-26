package metricshipper

import (
	"encoding/base64"
	"strings"
	"time"

	"github.com/rcrowley/go-metrics"
	"github.com/zenoss/glog"
	"github.com/zenoss/websocket"
)

var origin string = "http://localhost"

type WebsocketPublisher struct {
	config                   *websocket.Config
	concurrency              int
	conn                     chan *websocket.Conn
	batch_size               int
	batch_timeout            float64
	encoding                 string
	Outgoing                 chan Metric
	retry_connection         int
	retry_connection_timeout time.Duration //seconds
	max_connection_age       time.Duration // seconds
	OutgoingDatapoints       metrics.Meter // number of datapoints written to websocket endpoint
	OutgoingBytes            metrics.Meter // number of bytes written to websocket endpoint
	conn_ages                map[*websocket.Conn]time.Time
	conn_dicts               map[*websocket.Conn]*dictionary
}

func NewWebsocketPublisher(uri string, concurrency int, buffer_size int,
	batch_size int, batch_timeout float64, retry_connection int,
	retry_connection_timeout time.Duration, max_connection_age time.Duration,
	username string, password string, encoding string) (publisher *WebsocketPublisher, err error) {

	config, err := websocket.NewConfig(uri, origin)
	if err != nil {
		return nil, err
	}

	data := []byte(username + ":" + password)
	str := base64.StdEncoding.EncodeToString(data)
	config.Header.Add("Authorization", "basic "+str)

	outgoingDatapoints := metrics.NewMeter()
	metrics.Register("outgoingDatapoints", outgoingDatapoints)
	outgoingBytes := metrics.NewMeter()
	metrics.Register("outgoingBytes", outgoingBytes)

	return &WebsocketPublisher{
		config:                   config,
		concurrency:              concurrency,
		conn:                     make(chan *websocket.Conn, concurrency),
		batch_size:               batch_size,
		batch_timeout:            batch_timeout,
		encoding:                 encoding,
		Outgoing:                 make(chan Metric, buffer_size),
		retry_connection:         retry_connection,
		retry_connection_timeout: retry_connection_timeout,
		max_connection_age:       max_connection_age,
		OutgoingDatapoints:       outgoingDatapoints,
		OutgoingBytes:            outgoingBytes,
		conn_ages:                make(map[*websocket.Conn]time.Time),
		conn_dicts:               make(map[*websocket.Conn]*dictionary),
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
		delete(w.conn_ages, conn)
		delete(w.conn_dicts, conn)
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
			w.conn_ages[conn] = time.Now()
			w.conn_dicts[conn] = &dictionary{trans: make(map[string]int32)}
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

func (w *WebsocketPublisher) sendBatch(batch *MetricBatch) (metricCount, bytes int, err error) {
	var num int
	if batch != nil {
		num = len(batch.Metrics)
	}
	var dead *bool = new(bool)
	*dead = false
	conn := w.GetConn()
	glog.V(3).Infof("enter sendBatch(), conn=%s, len(batch)=%d", conn.Config().Location, len(batch.Metrics))
	defer glog.V(3).Infof("exit sendBatch(), dead=%t, num=%d", *dead, num)
	defer w.ReleaseConn(conn, dead)

	switch strings.ToLower(w.encoding) {
	case "json":
		bytes, err = websocket.JSON.Send(conn, batch)
	case "binary":
		msg, err := batch.MarshalBinary(w.conn_dicts[conn], true)
		if err != nil {
			return num, bytes, err
		}
		bytes, err = websocket.Message.Send(conn, msg)
	}
	if err != nil {
		*dead = true
		return num, bytes, err
	}

	*dead, err = w.readResponse(conn)

	// Allow the connection to die if older than the max age specified
	has_max_age := w.max_connection_age.Nanoseconds() == 0
	if has_max_age && time.Now().After(w.conn_ages[conn].Add(w.max_connection_age)) {
		glog.V(2).Infof("Connection is older than %d seconds; closing", w.max_connection_age.Seconds())
		*dead = true
	}

	return num, bytes, err
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
				metrics, bytes, err := w.sendBatch(batch)
				if err == nil {
					glog.V(2).Infof("Sent %d metrics to the consumer.", metrics)

					// update meter with number of metrics sent
					w.OutgoingDatapoints.Mark(int64(metrics))
					w.OutgoingBytes.Mark(int64(bytes))

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
