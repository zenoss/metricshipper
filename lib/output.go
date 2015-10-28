package metricshipper

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/rcrowley/go-metrics"
	"github.com/zenoss/glog"
	"github.com/zenoss/websocket"
)

var origin string = "http://localhost"

type WebsocketPublisher struct {
	pool               *WebSocketConnPool
	batch_size         int
	batch_timeout      float64
	encoding           string
	Outgoing           chan Metric
	OutgoingDatapoints metrics.Meter // number of datapoints written to websocket endpoint
	OutgoingBytes      metrics.Meter // number of bytes written to websocket endpoint
	ErrorDatapoints	   metrics.Meter
}

func NewWebsocketPublisher(uri string, concurrency int, buffer_size int,
	batch_size int, batch_timeout float64, retry_connection_timeout time.Duration,
	max_connection_age time.Duration, username string, password string,
	encoding string, window, maxcollisions, maxdelay int) (publisher *WebsocketPublisher, err error) {

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
	errorDataPoints := metrics.NewMeter()
	metrics.Register("errorDatapoints", errorDataPoints)

	pool := NewWebSocketConnPool(concurrency, retry_connection_timeout, max_connection_age, config)
	publisher = &WebsocketPublisher{
		pool:               pool,
		batch_size:         batch_size,
		batch_timeout:      batch_timeout,
		encoding:           encoding,
		Outgoing:           make(chan Metric, buffer_size),
		OutgoingDatapoints: outgoingDatapoints,
		OutgoingBytes:      outgoingBytes,
		ErrorDatapoints:	errorDataPoints,
	}

	// Block until at least one connection has been established
	pool.WaitForConnection()

	// Now it's cool to open the gates
	for i := 0; i < concurrency; i++ {
		go publisher.DoBatch(NewBackoff(window, maxcollisions, maxdelay))
	}
	return publisher, nil
}

func (w *WebsocketPublisher) getBatch() (int, *MetricBatch, *MetricBatch) {
	glog.V(3).Infof("enter getBatch()")
	buf := make([]Metric, 0)
	errorBuffer := make([]Metric, 0)
	batch := &MetricBatch{
		Metrics: buf,
	}
	errorBatch := &MetricBatch{
		Metrics: errorBuffer,
	}
	defer glog.V(3).Infof("exit getBatch(), len(buf)=%d", len(buf))

	remaining := w.batch_size - len(buf)
	timer := time.After(time.Duration(w.batch_timeout) * time.Second)
	for i := 0; i < remaining; i++ {
		select {
		case <-timer:
			i = remaining // Break out of the loop
		case m := <-w.Outgoing:
			if m.Error {
				errorBuffer = append(errorBuffer, m)
			} else {
				buf = append(buf, m)
			}
		}
	}
	batch.Metrics = buf
	errorBatch.Metrics = errorBuffer

	return len(buf), batch, errorBatch
}

func (w *WebsocketPublisher) sendBatch(batch *MetricBatch, backoff *Backoff) (metricCount, bytes int, err error) {
	var num int
	if batch != nil {
		num = len(batch.Metrics)
	}
	conn := w.pool.Get()
	defer w.pool.Put(conn)
	glog.V(3).Infof("enter sendBatch(), conn=%s, len(batch)=%d", w.pool.config.Location, len(batch.Metrics))
	defer glog.V(3).Infof("exit sendBatch(), num=%d", num)

	if glog.V(5) {
		for _, m := range batch.Metrics {
			glog.V(5).Infof("publishing: %+v", m)
		}
	}

	switch strings.ToLower(w.encoding) {
	case "json":
		bytes, err = websocket.JSON.Send(conn.conn, batch)
	case "binary":
		msg, err := batch.MarshalBinary(conn.dictionary, true)
		if err != nil {
			return 0, bytes, err
		}
		bytes, err = websocket.Message.Send(conn.conn, msg)
	}
	if err != nil {
		conn.Close()
		return num, bytes, err
	}
	return num, bytes, w.readResponse(conn, backoff)
}

var bufferPool = &sync.Pool{
	New: func() interface{} {
		return make([]byte, 1024)
	},
}

//read everything in the response buffer
func (w *WebsocketPublisher) readResponse(conn *WebSocketConn, backoff *Backoff) (err error) {
	msg := bufferPool.Get().([]byte)
	defer bufferPool.Put(msg)
	for {
		n := 0
		deadline := time.Now().Add(time.Microsecond)
		err = conn.conn.SetReadDeadline(deadline)

		if n, err = conn.conn.Read(msg); err != nil && !strings.HasSuffix(err.Error(), "i/o timeout") {
			conn.Close()
			break
		}

		err = nil
		if n == 0 {
			break
		}
		dmsg := make(map[string]string)
		if err := json.Unmarshal(msg[0:n], &dmsg); err != nil {
			return err
		}
		if strings.HasSuffix(dmsg["type"], "COLLISION") || dmsg["type"] == "DROPPED" {
			backoff.Collision()
		}
		glog.V(2).Infof("Server responded with message: %v", dmsg)
	}
	return err
}

func (w *WebsocketPublisher) DoBatch(backoff *Backoff) {
	for {
		// Retry loop
		num, batch, errorBatch := w.getBatch()
		if num > 0 {
			for {
				backoff.Wait()
				metrics, bytes, err := w.sendBatch(batch, backoff)
				if err == nil {
					glog.V(2).Infof("Sent %d metrics to the consumer.", metrics)

					// update meter with number of metrics sent
					w.OutgoingDatapoints.Mark(int64(metrics))
					w.OutgoingBytes.Mark(int64(bytes))
					if errorBatch != nil {
						w.ErrorDatapoints.Mark(int64(len(errorBatch.Metrics)))
					}

					break
				} else {
					glog.Errorf("Failed sending %d metrics to the consumer: %s", num, err)
				}
			}
		}
	}
}
