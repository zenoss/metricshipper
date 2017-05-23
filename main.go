package main

import (
	"github.com/zenoss/glog"
	"github.com/zenoss/metricshipper/lib"

	"os"
	"runtime"
	"time"

	"github.com/control-center/serviced/logging"
)

var (
	plog = logging.PackageLogger()
)

func naive_pluralize(i int, word string) string {
	if i == 1 {
		return word
	} else {
		return word + "s"
	}
}

func numProcs(c *metricshipper.ShipperConfig) int {
	max := runtime.NumCPU()
	if c.CPUs == 0 || c.CPUs > max {
		return max
	}
	return c.CPUs
}

func main() {
	plog.Info("begin main()")
	// Get us some configuration
	config, err := metricshipper.ParseShipperConfig()
	if err != nil {
		glog.Errorf("Unable to parse config: %s", err)
		os.Exit(1)
	}

	// Adjust parallelism to specified values or default to number of
	// available logical CPUs
	num := numProcs(config)
	glog.Infof("Using %d %s", num, naive_pluralize(num, "processor"))
	plog.WithField("numprocs", num).Info("starting")
	runtime.GOMAXPROCS(num)

	// First, connect to the websocket
	glog.Infof("Initiating %d %s to consumer", config.Writers,
		naive_pluralize(config.Writers, "connection"))
	w, err := metricshipper.NewWebsocketPublisher(config.ConsumerUrl,
		config.Readers, config.MaxBufferSize, config.MaxBatchSize,
		config.BatchTimeout, time.Duration(config.RetryConnectionTimeout)*time.Second,
		time.Duration(config.MaxConnectionAge)*time.Second, config.Username, config.Password, config.Encoding,
		config.BackoffWindow, config.MaxBackoffSteps, config.MaxBackoffDelay)
	if err != nil {
		glog.Error("Unable to create WebSocket forwarder")
		return
	}

	// Next, try to connect to Redis
	glog.Infof("Initiating %d %s to redis", config.Readers,
		naive_pluralize(config.Readers, "connection"))
	plog.WithField("numreaders", config.Readers).
		Info("Initiating connection(s) to redis")
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

	// Create a stats reporter and start it
	glog.Info("Warming up the stats reporter")
	s := &metricshipper.MetricStats{
		MetricsChannel:       &r.Incoming,
		IncomingMeter:        &r.IncomingMeter,
		OutgoingMeter:        &w.OutgoingDatapoints,
		OutgoingBytes:        &w.OutgoingBytes,
		StatsInterval:        config.StatsInterval,
		ErrorsMeter:          &w.ErrorDatapoints,
		ControlPlaneStatsURL: os.Getenv("CONTROLPLANE_CONSUMER_URL"),
	}
	go s.Start()

	// Finally, open the Redis floodgates (also manages own goroutines)
	glog.Info("Subscribing to metrics queue")
	r.Subscribe()
}
