package metricshipper

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/zenoss/websocket"
)

var serverAddr string
var once sync.Once
var buf []string = make([]string, 0)

func clearBuffer() {
	buf = buf[:0]
}

func getMetric() (metric *Metric) {
	var buffer bytes.Buffer
	now := strconv.Itoa(int(time.Now().Unix()))
	buffer.WriteString("{\"timestamp\":")
	buffer.WriteString(now)
	buffer.WriteString(", \"metric\": \"test\", \"value\":1234,")
	buffer.WriteString("\"tags\":{\"device\":\"ian\"}}")
	metric, _ = MetricFromJSON(buffer.Bytes())
	return metric
}

func stageMetrics(num int, pub *WebsocketPublisher) {
	for i := 0; i < num; i++ {
		pub.Outgoing <- *getMetric()
	}
}

func consumerHandler(ws *websocket.Conn) {
	msg := make([]byte, 1024)
	n, _ := ws.Read(msg)
	msg = msg[0:n]
	buf = append(buf, string(msg))
}

func assertBufferSize(expected int, msg string, t *testing.T) {
	actual := len(buf)
	if actual != expected {
		t.Error(msg)
	}
}

// Echo the data received on the WebSocket.
func startServer() {
	http.Handle("/metrics", websocket.Handler(consumerHandler))
	server := httptest.NewServer(nil)
	serverAddr = server.Listener.Addr().String()
	log.Print("Test server listening on ", serverAddr)
}

func TestConnectFail(t *testing.T) {
	pub, err := NewWebsocketPublisher("ws://127.0.0.1:12345/metrics", 1, 1, 1, 1, 1, 1, 999, "admin", "zenoss", false)
	if err != nil {
		t.Fatalf("Could not create websocket publisher: %s", err)
	}
	if err := pub.Start(); err == nil {
		t.Errorf("Unable to connect: %s", err)
	}
}

func TestConnect(t *testing.T) {
	once.Do(startServer)
	defer clearBuffer()
	pub, err := NewWebsocketPublisher("ws://"+serverAddr+"/metrics", 1, 1, 1, 1, 1, 1, 999, "admin", "zenoss", false)
	if err != nil {
		t.Fatalf("Could not create websocket publisher: %s", err)
	}
	if err := pub.Start(); err != nil {
		t.Error("Unable to connect: %s", err)
	}
}

func TestPublishOne(t *testing.T) {
	once.Do(startServer)
	defer clearBuffer()
	pub, err := NewWebsocketPublisher("ws://"+serverAddr+"/metrics", 1, 1, 1, 1, 1, 1, 999, "admin", "zenoss", false)
	if err != nil {
		t.Fatalf("Could not create websocket publisher: %s", err)
	}
	go pub.Start()
	stageMetrics(1, pub)
	time.Sleep(5 * time.Millisecond)
	assertBufferSize(1, "Didn't receive single metric", t)
}

func TestHitBatchSize(t *testing.T) {
	once.Do(startServer)
	defer clearBuffer()
	pub, err := NewWebsocketPublisher("ws://"+serverAddr+"/metrics", 1, 6, 3, 1, 1, 1, 999, "admin", "zenoss", false)
	if err != nil {
		t.Fatalf("Could not create websocket publisher: %s", err)
	}
	go pub.Start()
	stageMetrics(2, pub)
	time.Sleep(5 * time.Millisecond)
	assertBufferSize(0, "Sent batch early", t)
	stageMetrics(1, pub)
	time.Sleep(5 * time.Millisecond)
	assertBufferSize(1, "Didn't send batch on time", t)
}

func TestHitBatchTimeout(t *testing.T) {
	once.Do(startServer)
	defer clearBuffer()
	pub, err := NewWebsocketPublisher("ws://"+serverAddr+"/metrics", 1, 6, 3, 1, 1, 1, 999, "admin", "zenoss", false)
	if err != nil {
		t.Fatalf("Could not create websocket publisher: %s", err)
	}
	go pub.Start()
	stageMetrics(1, pub)
	time.Sleep(5 * time.Millisecond)
	assertBufferSize(0, "Sent batch early", t)
	time.Sleep(1 * time.Second)
	assertBufferSize(1, "Didn't send batch after 1 second", t)
}
