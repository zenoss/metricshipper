package metricshipper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/rcrowley/go-metrics"
	"github.com/zenoss/glog"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

// MetricStats publishes and reports internal metrics
type MetricStats struct {
	MetricsChannel       *chan Metric
	IncomingMeter        *metrics.Meter
	OutgoingMeter        *metrics.Meter
	StatsInterval        int
	ControlPlaneStatsURL string

	tags map[string]interface{}
}

// Start starts the stats publishing/reporting
func (ms *MetricStats) Start() {
	ms.tags = make(map[string]interface{})
	ms.tags["host"] = os.Getenv("CONTROLPLANE_HOST_ID")

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
	metrics = append(metrics, generateMeterMetrics(ms.IncomingMeter, "totalIncoming", ms.tags)...)
	metrics = append(metrics, generateMeterMetrics(ms.OutgoingMeter, "totalOutgoing", ms.tags)...)

	// update incoming meter with number of metrics to match outgoing since
	// we are injecting these metrics onto the incoming queue
	(*ms.IncomingMeter).Mark(int64(len(metrics)))

	// publish internal metrics to consumer
	for _, met := range metrics {
		*ms.MetricsChannel <- met
		if glog.V(3) {
			glog.Infof("METRIC INT %+v", met)
		}
	}

	// publish internal metrics to controlplane
	//   data='{"metrics": [{"timestamp":1.401204588e+09,"metric":"ZEN_INF.org.zenoss.app.metricshipper.totalIncoming.count","value":1,"tags":{"host":"570a276e"}}]}'
	//   curl -s -XPOST -H "Content-Type: application/json" -d "$data" $CONTROLPLANE_CONSUMER_URL
	if len(ms.ControlPlaneStatsURL) > 0 {
		post := func(url string, statsBytes []byte) error {
			glog.V(3).Infof("Posting stats to %s: %s\n", url, string(statsBytes))
			resp, err := http.Post(url, "application/json", bytes.NewReader(statsBytes))
			if err != nil {
				return fmt.Errorf("posting stats to %s: %s %s", url, string(statsBytes), err)
			}
			glog.V(4).Infof("Response %v\n", resp)
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			glog.V(4).Infof("Post result %s\n", string(body))
			if err != nil {
				return err
			}
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				return fmt.Errorf("response %d posting to %s: %s", resp.StatusCode, url, string(body))
			}
			return nil
		}

		stats := map[string][]Metric{}
		stats["metrics"] = metrics
		statsBytes, err := json.Marshal(stats)
		if err != nil {
			glog.Errorf("Unable to json.Marshal stats: %+v", stats)
			return
		}

		err = post(ms.ControlPlaneStatsURL, statsBytes)
		if err != nil {
			glog.Errorf("%s", err)
			return
		}
	}
}

// generateMeterMetrics creates a slice of Metrics from a meter and name
func generateMeterMetrics(meter *metrics.Meter, infix string, tags map[string]interface{}) []Metric {
	prefix := fmt.Sprintf("ZEN_INF.org.zenoss.app.metricshipper.%s", infix)

	metrics := []Metric{}
	metrics = append(metrics, toMetric(fmt.Sprintf("%s.count", prefix), float64((*meter).Count()), tags))
	metrics = append(metrics, toMetric(fmt.Sprintf("%s.meanRate", prefix), (*meter).RateMean(), tags))
	metrics = append(metrics, toMetric(fmt.Sprintf("%s.1MinuteRate", prefix), (*meter).Rate1(), tags))
	metrics = append(metrics, toMetric(fmt.Sprintf("%s.5MinuteRate", prefix), (*meter).Rate5(), tags))
	metrics = append(metrics, toMetric(fmt.Sprintf("%s.15MinuteRate", prefix), (*meter).Rate15(), tags))

	glog.V(1).Infof("INTERNAL %s: %10.0f %9.1f/s %8.1f/1m %8.1f/5m %8.1f/15m",
		infix, metrics[0].Value, metrics[1].Value, metrics[2].Value, metrics[3].Value, metrics[4].Value)

	return metrics
}

// toMetric creates a Metric from a name and value
func toMetric(name string, value float64, tags map[string]interface{}) Metric {
	metric := Metric{}
	metric.Metric = name
	metric.Timestamp = float64(time.Now().Unix())
	metric.Value = value
	metric.Tags = tags
	return metric
}
