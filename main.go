package main

import (
	"flag"
	"github.com/zenoss/glog"
	"github.com/zenoss/metricshipper/lib"
)

var args struct {
	redisUrl        string
	readers         int
	consumerUrl     string
	writers         int
	maxBufferSize   int
	maxBatchSize    int
	batchTimeout    float64
	backoffWindow   int
	maxBackoffSteps int
}

func init() {
	flag.StringVar(&args.redisUrl, "redis-url", "redis://localhost:6379/0/metrics", "Redis URL to subscribe to")
	flag.IntVar(&args.readers, "readers", 2, "Maximum number of simultaneous readers from Redis")
	flag.StringVar(&args.consumerUrl, "consumer-url", "ws://localhost:9090/publish", "WebSocket URL of consumer to publish to")
	flag.IntVar(&args.writers, "writers", 1, "Maximum number of simultaneous writers to the consumer")
	flag.IntVar(&args.maxBufferSize, "max-buffer-size", 1024, "Maximum number of messages to keep in the internal buffer")
	flag.IntVar(&args.maxBatchSize, "max-batch-size", 128, "Number of messages to send to the consumer in a single call. This should be smaller than the buffer size.")
	flag.Float64Var(&args.batchTimeout, "batch-timeout", 0.1, "Maximum time in seconds to wait for messages from the internal buffer to be ready before making a web socket call with current metrics")
	flag.IntVar(&args.backoffWindow, "backoff-window", 60, "Rolling time period in seconds to consider collision messages from the consumer")
	flag.IntVar(&args.maxBackoffSteps, "max-backoff-steps", 16, "Maximum number of collisions to consider for exponential backoff")
	flag.Parse()
}

func main() {
	glog.Info("Initiating 1 connection to consumer")
	// First, connect to the websocket
	w, err := metricshipper.NewWebsocketPublisher(args.consumerUrl,
		args.readers, args.maxBufferSize, args.maxBatchSize)
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
	r, err := metricshipper.NewRedisReader(args.redisUrl, 128, 1024, 1)
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
