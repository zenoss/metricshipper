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
			processed.Error = true
			continue
		} else {
			processed.Error = false
		}

		*m.Outgoing <- *processed
	}
}

func (m *MetricProcessor) Process(metric *Metric) (met *Metric, err error) {
	glog.V(1).Infof("MetricProcessor.Process() mtrace flag mtraceEnabled = %t", mtraceEnabled)
	// POLICY GOES HERE
	if met == nil {
		glog.V(2).Infof("MetricProcessor.Process(): nil metric passed in")
	} else if mtraceEnabled && met.HasTracer() {
		met.TracerMessage("Process()")
	}
	return metric, nil
}
