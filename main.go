package main

import (
	"github.com/zenoss/glog"
	"github.com/zenoss/metricshipper/lib"
	"os"
)

func naive_pluralize(i int, word string) string {
	if i == 1 {
		return word
	} else {
		return word + "s"
	}
}

func main() {
	// Get us some configuration
	config, err := metricshipper.ParseShipperConfig()
	if err != nil {
		os.Exit(1)
	}

	// First, connect to the websocket
	glog.Infof("Initiating %d %s to consumer", config.Writers,
		naive_pluralize(config.Writers, "connection"))
	w, err := metricshipper.NewWebsocketPublisher(config.ConsumerUrl,
		config.Readers, config.MaxBufferSize, config.MaxBatchSize,
		config.BatchTimeout, config.Username, config.Password)
	if err != nil {
		glog.Error("Unable to create WebSocket forwarder")
		return
	}
	// Websocket forwarder manages its own goroutines
	err = w.Start()
	if err != nil {
		glog.Error("Unable to start WebSocket forwarder")
		return
	}

	// Next, try to connect to Redis
	glog.Infof("Initiating %d %s to redis", config.Readers,
		naive_pluralize(config.Readers, "connection"))
	r, err := metricshipper.NewRedisReader(config.RedisUrl, config.MaxBatchSize,
		config.MaxBufferSize, config.Readers)
	if err != nil {
		glog.Error("Unable to create Redis reader")
		return
	}

	// Create a processor and start it going
	glog.Info("Warming up the processor")
	p := &metricshipper.MetricProcessor{
		Incoming: &r.Incoming,
		Outgoing: &w.Outgoing,
	}
	go p.Start()

	// Finally, open the Redis floodgates (also manages own goroutines)
	glog.Info("Subscribing to metrics queue")
	r.Subscribe()
}
