package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/zenoss/glog"
	flags "github.com/zenoss/go-flags"
	"net/http"
	"sync/atomic"
)

// faux metric consumer config parameters
type ConsumerConfig struct {
	Port int `long:"port" short:"p" description:"What websocket port to listen for metrics" default:"8443"`
}

// parse faux consumer arguments
func parse_consumer_args(args []string) (ConsumerConfig, error) {
	config := ConsumerConfig{}
	parser := flags.NewParser(&config, flags.Default)
	_, err := parser.Parse()
	return config, err
}

//running metric count
var total int32 = 0

func consumerHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Upgrade(w, r, nil, 4096, 4096)
	if err != nil {
		glog.Errorf("Failed websocket.Upgrade():", err)
		http.Error(w, "Bad request", 400)
		return
	}
	defer conn.Close()

	for {
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			glog.Errorf("Failed reading message: %s", err)
			break
		}

		if messageType == websocket.TextMessage {
			message := Message{}
			err := json.Unmarshal(payload, &message)
			if err == nil {
				var length int32 = int32(len(message.Metrics))
				glog.Infof("len(Message.Metrics)=%d, total=%d", length, atomic.AddInt32(&total, length))
				ok := Control{Type: "OK"}
				response, _ := json.Marshal(ok)
				conn.WriteMessage(websocket.TextMessage, response)
			} else {
				glog.Errorf("Failed to unmarshal payload: %s", err)
			}
		}
	}
}

// faux consumer
func consumer(args []string) {
	config, err := parse_consumer_args(args)
	if err != nil {
		return
	}
	http.HandleFunc("/ws/metrics/store", consumerHandler)
	err = http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", config.Port), nil)
	if err != nil {
		panic("ListenAndServe: " + err.Error())
	}
}
