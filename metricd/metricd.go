package main

import (
	"flag"
	"github.com/iancmcc/metricd/input"
	"github.com/iancmcc/metricd/output"
	"github.com/iancmcc/metricd/processor"
	"github.com/zenoss/glog"
)

func main() {
	flag.Parse()

	glog.Info("Initiating 1 connection to consumer")
	// First, connect to the websocket
	w, err := output.NewWebsocketPublisher("ws://localhost:8080/ws/metrics/store", 1, 1024, 64)
	if err != nil {
		glog.Fatal("Unable to create WebSocket forwarder")
		return
	}
	err = w.Start()
	if err != nil {
		glog.Fatal("Unable to start WebSocket forwarder")
		return
	}
	// Next, try to talk to redis
	glog.Info("Initiating 2 connections to redis")
	r, err := input.NewRedisReader("redis://127.0.0.1:6379/0/metrics", 128, 1024, 1)
	if err != nil {
		glog.Fatal("Unable to create redis reader")
		return
	}
	// Create a processor and start it going
	glog.Info("Warming up the processor")
	p := &processor.MetricProcessor{
		Incoming: &r.Incoming,
		Outgoing: &w.Outgoing,
	}
	go p.Start()

	// Finally, open the redis floodgates
	glog.Info("Subscribing to metrics queue")
	r.Subscribe()
}