package metricd

type MetricProcessor struct {
	Incoming *chan Metric
	Outgoing *chan Metric
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

func (m *MetricProcessor) Process(metric *Metric) (met *Metric, err error) {
	// POLICY GOES HERE
	return metric, nil
}
