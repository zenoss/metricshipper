package metricshipper

import (
	"time"

	"github.com/zenoss/glog"
	"github.com/zenoss/websocket"
)

type WebSocketConn struct {
	conn       *websocket.Conn // The underlying connection
	expires    time.Time       // The expiration time of this connection
	dictionary *dictionary     // Translation dictionary for binary encoding
	closed     bool
}

func (conn *WebSocketConn) Close() {
	conn.closed = true
	if err := conn.conn.Close(); err != nil {
		glog.V(1).Infof("Unable to close WebSocket connection: (%v)", err)
	}
}

type WebSocketConnPool struct {
	delay   time.Duration
	maxage  time.Duration
	config  *websocket.Config
	pool    chan *WebSocketConn
	discard chan *WebSocketConn
}

func (pool *WebSocketConnPool) newWebSocket() *WebSocketConn {
	for {
		if conn, err := websocket.DialConfig(pool.config); err != nil {
			glog.Infof("Unable to connect to consumer %s", pool.config.Location)
			time.Sleep(pool.delay)
			continue
		} else {
			glog.Infof("Connected to consumer %s", pool.config.Location)
			var expires time.Time
			// Don't expire connections if maxage is 0
			if pool.maxage > 0 {
				expires = time.Now().Add(pool.maxage)
			}
			return &WebSocketConn{
				conn:       conn,
				expires:    expires,
				dictionary: &dictionary{trans: make(map[string]int32)},
			}
		}
	}
}

func NewWebSocketConnPool(size int, delay time.Duration, maxage time.Duration, config *websocket.Config) *WebSocketConnPool {
	pool := &WebSocketConnPool{
		delay:  delay,
		maxage: maxage,
		config: config,
		pool:   make(chan *WebSocketConn, size),
	}
	go func() {
		for i := 0; i < size; i++ {
			pool.pool <- pool.newWebSocket()
		}
	}()
	return pool
}

func (pool *WebSocketConnPool) WaitForConnection() {
	pool.pool <- <-pool.pool
}

func (pool *WebSocketConnPool) Get() *WebSocketConn {
	return <-pool.pool
}

func (pool *WebSocketConnPool) Put(conn *WebSocketConn) {
	if conn.closed {
		pool.Release(conn)
	} else if !conn.expires.IsZero() && time.Now().After(conn.expires) {
		glog.V(2).Infof("Connection is older than %d seconds; closing", pool.maxage.Seconds())
		pool.Release(conn)
	} else {
		pool.pool <- conn
	}
}

func (pool *WebSocketConnPool) Release(conn *WebSocketConn) {
	defer conn.conn.Close()
	go func() {
		pool.pool <- pool.newWebSocket()
	}()
}
