package metricshipper

import (
	"encoding/json"
)

type PublisherError struct {
	Msg string
}

func (s PublisherError) Error() string {
	return s.Msg
}

// Defines the structure of a Metric message
type Metric struct {
	Timestamp float64 `json:"timestamp"`
	Metric    string  `json:"metric"`
	Value     float64 `json:"value"`
	Tags      struct {
		Device string `json:"device"`
	} `json:"tags"`
}

// Structure of message forwarded via websocket
type MetricBatch struct {
	Control interface{} `json:"control"` // Should be nil
	Metrics []Metric    `json:"metrics"`
}

// Convert a JSON-serialized metric into an instance
func MetricFromJSON(s []byte) (*Metric, error) {
	m := &Metric{}
	err := json.Unmarshal(s, m)
	if err != nil {
		return nil, err
	}
	return m, nil
}