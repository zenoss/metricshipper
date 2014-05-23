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
}

// Start starts the stats publishing/reporting
func (ms *MetricStats) Start() {
	// publish every second
	go func() {
		for {
			ms.PublishInternalMetrics()

			time.Sleep(1 * time.Second)
		}
	}()
}

// PublishInternalMetrics publishes internal metrics
func (ms *MetricStats) PublishInternalMetrics() {
	glog.V(2).Infof("enter MetricStats.PublishInternalMetrics()")
	defer glog.V(2).Infof("exit MetricStats.PublishInternalMetrics()")

	incomingMetrics := generateMeterMetrics(ms.IncomingMeter, "totalIncoming")
	for _, met := range incomingMetrics {
		*ms.MetricsChannel <- met
		if glog.V(1) {
			glog.Infof("METRIC INT %+v", met)
		}
	}

	// TODO: report outgoing metrics
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
