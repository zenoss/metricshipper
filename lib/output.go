package metricshipper

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
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
}

func NewWebsocketPublisher(uri string, concurrency int, buffer_size int,
	batch_size int, batch_timeout float64, retry_connection_timeout,
	max_connection_age time.Duration, username, password,
	encoding string) (publisher *WebsocketPublisher, err error) {

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

	pool := NewWebSocketConnPool(concurrency, retry_connection_timeout, max_connection_age, config)
	publisher = &WebsocketPublisher{
		pool:               pool,
		batch_size:         batch_size,
		batch_timeout:      batch_timeout,
		encoding:           encoding,
		Outgoing:           make(chan Metric, buffer_size),
		OutgoingDatapoints: outgoingDatapoints,
		OutgoingBytes:      outgoingBytes,
	}

	// Block until at least one connection has been established
	pool.WaitForConnection()

	// Now it's cool to open the gates
	for i := 0; i < concurrency; i++ {
		go publisher.DoBatch()
	}
	return publisher, nil
}

var batchLimit = func(w *WebsocketPublisher, conn *WebSocketConn) int {
	if conn.receiveBuffer == 0 {
		return 0
	}
	if conn.receiveBuffer > int64(w.batch_size) {
		return w.batch_size
	}
	return int(conn.receiveBuffer)
}

func (w *WebsocketPublisher) getBatch(conn *WebSocketConn) *MetricBatch {
	buf := make([]Metric, 0)
	batch := &MetricBatch{
		Metrics: buf,
	}
	limit := batchLimit(w, conn)
	timer := time.After(time.Duration(w.batch_timeout) * time.Second)
	for i := 0; i < limit; i++ {
		select {
		case <-timer:
			i = limit // Break out of the loop
		case m := <-w.Outgoing:
			buf = append(buf, m)
		}
	}
	batch.Metrics = buf
	return batch
}

func (w *WebsocketPublisher) sendBatch(conn *WebSocketConn, batch *MetricBatch) (metricCount, bytes int, err error) {
	num := len(batch.Metrics)
	glog.V(3).Infof("enter sendBatch(), conn=%s, len(batch)=%d", w.pool.config.Location, num)
	defer glog.V(3).Infof("exit sendBatch(), len(batch)=%d", num)

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
			return 0, 0, err
		}
		bytes, err = websocket.Message.Send(conn.conn, msg)
	}
	if err != nil {
		conn.Close()
		return num, bytes, err
	}
	return num, bytes, nil
}

var bufferPool = &sync.Pool{
	New: func() interface{} {
		return make([]byte, 1024)
	},
}

var readWebsocket = func(msg []byte, conn *WebSocketConn) (int, error) {
	deadline := time.Now().Add(10 * time.Microsecond)
	if err := conn.conn.SetReadDeadline(deadline); err != nil {
		conn.Close()
		return 0, err
	}

	if n, err := conn.conn.Read(msg); err != nil {
		if strings.HasSuffix(err.Error(), "i/o timeout") {
			return 0, nil
		} else {
			conn.Close()
			return 0, err
		}
	} else {
		return n, err
	}
}

//read everything in the response buffer
func (w *WebsocketPublisher) readResponse(conn *WebSocketConn, nonce string) (err error) {
	msg := bufferPool.Get().([]byte)
	defer bufferPool.Put(msg)
	for {
		n := 0
		if n, err = readWebsocket(msg, conn); err != nil || n == 0 {
			break
		}
		dmsg := make(map[string]string)
		if err := json.Unmarshal(msg[0:n], &dmsg); err != nil {
			return err
		}
		logged := false
		switch dmsg["type"] {
		case "OK":
			//TODO: Switch to async
		case "ERROR":
			glog.Errorf("Server responded with message: %v", dmsg)
			logged = true
			conn.Close()
			return errors.New("Server error: " + dmsg["value"])
		case "DROPPED":
			glog.Errorf("Server responded with message: %v", dmsg)
			logged = true
			//TODO: Switch to sync
		case "MALFORMED_REQUEST":
			glog.Errorf("Server responded with message: %v", dmsg)
			logged = true
		case "DATA_RECEIVED":
			// ignored
		case "BUFFER_UPDATE":
			if bufferUpdate, err := strconv.ParseInt(dmsg["value"], 10, 64); err == nil {
				conn.receiveBuffer = bufferUpdate
			} else {
				glog.Errorf("Buffer update parse error: %v", err)
			}
		case "PONG":
			//TODO: Ignore in async mode. Otherwise...
			//TODO: Stop looping and return if value matches our nonce.
		}
		if !logged {
			glog.V(2).Infof("Server responded with message: %v", dmsg)
		}
	}
	return err
}

var emptyBatch = &MetricBatch{
	Metrics: make([]Metric, 0),
}

var pollForFreeBuffer = func(conn *WebSocketConn) bool {
	return conn.receiveBuffer <= 0
}

func (w *WebsocketPublisher) getAndSendOneBatch() (metricCount, byteCount int) {
	glog.V(3).Infof("enter getAndSendOneBatch()")
	defer glog.V(3).Infof("exit getAndSendOneBatch()")

	conn := w.pool.Get()
	defer w.pool.Put(conn)

	batch := w.getBatch(conn)
	num := len(batch.Metrics)

	if pollForFreeBuffer(conn) || num > 0 {
		// Retry loop...
		for {
			if pollForFreeBuffer(conn) {
				_, _, err := w.sendBatch(conn, emptyBatch)
				if err == nil {
					err = w.readResponse(conn, "")
				}
				if err != nil {
					glog.Errorf("Failed polling consumer for a buffer update: %s", err)
				} else {
					glog.V(2).Info("Polled consumer for buffer update.")
				}
				if pollForFreeBuffer(conn) {
					<-time.After(100 * time.Millisecond) // TODO: make this configurable
				}
			} else {
				metrics, bytes, err := w.sendBatch(conn, batch)
				if err == nil {
					conn.receiveBuffer -= int64(metrics)
					err = w.readResponse(conn, "")
				}
				if err == nil {
					return metrics, bytes
				} else {
					glog.Errorf("Failed sending %d metrics to the consumer: %s", num, err)
					//TODO: split the batch if 0 < conn.receiveBuffer < len(batch)
				}
			}
		}
	}
	return 0, 0
}

func (w *WebsocketPublisher) DoBatch() {
	for {
		metrics, bytes := w.getAndSendOneBatch()
		glog.V(2).Infof("Sent %d metrics to the consumer.", metrics)
		w.OutgoingDatapoints.Mark(int64(metrics))
		w.OutgoingBytes.Mark(int64(bytes))
	}
}
