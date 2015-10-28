package metricshipper
import (
	"github.com/zenoss/glog"
)

type MetricProcessor struct {
	Incoming *chan Metric
	Outgoing *chan Metric
}

func (m *MetricProcessor) Start() {
	for {
		metric := <-*m.Incoming

		processed, err := m.Process(&metric)
		if err != nil {
			glog.V(3).Infof("There was an error processing a metric")
			processed.Status = 0
		} else {
			processed.Status = 1
		}

		*m.Outgoing <- *processed
	}
}

func (m *MetricProcessor) Process(metric *Metric) (met *Metric, err error) {
	// POLICY GOES HERE
	return metric, nil
}
