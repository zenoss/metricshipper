package metricshipper

import (
	"fmt"
	"github.com/rcrowley/go-metrics"
	"github.com/zenoss/glog"
	"time"
)

// MetricStats publishes and reports internal metrics
type MetricStats struct {
	MetricsChannel *chan Metric
	IncomingMeter  *metrics.Meter
	OutgoingMeter  *metrics.Meter
	StatsInterval  int
}

// Start starts the stats publishing/reporting
func (ms *MetricStats) Start() {
	// publish every second
	go func() {
		for {
			ms.PublishInternalMetrics()

			time.Sleep(time.Duration(ms.StatsInterval) * time.Second)
		}
	}()
}

// PublishInternalMetrics publishes internal metrics
func (ms *MetricStats) PublishInternalMetrics() {
	metrics := []Metric{}
	metrics = append(metrics, generateMeterMetrics(ms.IncomingMeter, "totalIncoming")...)
	metrics = append(metrics, generateMeterMetrics(ms.OutgoingMeter, "totalOutgoing")...)

	// report internal metrics
	for _, met := range metrics {
		*ms.MetricsChannel <- met
		if glog.V(3) {
			glog.Infof("METRIC INT %+v", met)
		}
	}
}

// generateMeterMetrics creates a slice of Metrics from a meter and name
func generateMeterMetrics(meter *metrics.Meter, infix string) []Metric {
	prefix := fmt.Sprintf("ZEN_INF.org.zenoss.app.metricshipper.%s", infix)

	metrics := []Metric{}
	metrics = append(metrics, toMetric(fmt.Sprintf("%s.count", prefix), float64((*meter).Count())))
	metrics = append(metrics, toMetric(fmt.Sprintf("%s.meanRate", prefix), (*meter).RateMean()))
	metrics = append(metrics, toMetric(fmt.Sprintf("%s.1MinuteRate", prefix), (*meter).Rate1()))
	metrics = append(metrics, toMetric(fmt.Sprintf("%s.5MinuteRate", prefix), (*meter).Rate5()))
	metrics = append(metrics, toMetric(fmt.Sprintf("%s.15MinuteRate", prefix), (*meter).Rate15()))

	glog.V(1).Infof("INTERNAL %s: %10.0f %9.1f/s %8.1f/1m %8.1f/5m %8.1f/15m",
		infix, metrics[0].Value, metrics[1].Value, metrics[2].Value, metrics[3].Value, metrics[4].Value)

	return metrics
}

// toMetric creates a Metric from a name and value
func toMetric(name string, value float64) Metric {
	metric := Metric{}
	metric.Metric = name
	metric.Timestamp = float64(time.Now().Unix())
	metric.Value = value
	metric.Tags = make(map[string]interface{})
	return metric
}
