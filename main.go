package main

import (
	"github.com/imdario/mergo"
	"github.com/zenoss/glog"
	flags "github.com/zenoss/go-flags"
	"github.com/zenoss/metricshipper/lib"
	"os"
)

var (
	config *metricshipper.ShipperConfig
)

func parseShipperOpts(cfg *metricshipper.ShipperConfig) error {
	parser := flags.NewParser(cfg, flags.Default|flags.IgnoreDefaults)
	_, err := parser.Parse()
	return err
}

func parseConfigFile(filename string, cfg *metricshipper.ShipperConfig) {
	file, err := os.Open(filename)
	if err == nil {
		metricshipper.LoadConfig(file, cfg)
	}
}

func init() {
	defaults := &metricshipper.ShipperConfig{}
	flags.ParseArgs(defaults, make([]string, 0))

	config = &metricshipper.ShipperConfig{}
	// Exit if help or parser error.
	if err := parseShipperOpts(config); err != nil {
		os.Exit(1)
	}

	fileconfig := &metricshipper.ShipperConfig{}
	parseConfigFile(config.ConfigFilePath, fileconfig)

	mergo.Merge(config, *fileconfig)
	mergo.Merge(config, *defaults)
}

func main() {
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
