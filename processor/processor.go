package processor

import (
	"github.com/iancmcc/metricd"
)

type MetricProcessor struct {
	Incoming *chan metricd.Metric
	Outgoing *chan metricd.Metric
}

func (m *MetricProcessor) Start() {
	for {
		metric := <-*m.Incoming

		processed, err := m.Process(&metric)
		if err != nil {
			// Log
			continue
		}
		*m.Outgoing <- *processed
	}
}

func (m *MetricProcessor) Process(metric *metricd.Metric) (met *metricd.Metric, err error) {
	// POLICY GOES HERE
	return metric, nil
}
