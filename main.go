package main

import (
	"github.com/zenoss/glog"
	"github.com/zenoss/metricshipper/lib"
	"os"
)

func main() {
	config, err := metricshipper.ParseShipperConfig()
	if err != nil {
		os.Exit(1)
	}

	glog.Info("Initiating 1 connection to consumer")
	// First, connect to the websocket
	w, err := metricshipper.NewWebsocketPublisher(config.ConsumerUrl,
		config.Readers, config.MaxBufferSize, config.MaxBatchSize)
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
	r, err := metricshipper.NewRedisReader(config.RedisUrl, 128, 1024, 1)
	if err != nil {
		glog.Fatal("Unable to create redis reader")
		return
	}
	// Create a processor and start it going
	glog.Info("Warming up the processor")
	p := &metricshipper.MetricProcessor{
		Incoming: &r.Incoming,
		Outgoing: &w.Outgoing,
	}
	go p.Start()

	// Finally, open the redis floodgates
	glog.Info("Subscribing to metrics queue")
	r.Subscribe()
}
