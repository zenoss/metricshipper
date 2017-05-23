package metricshipper

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/control-center/serviced/logging"
)

var (
	plog = logging.PackageLogger()
)

type PublisherError struct {
	Msg string
}

func (s PublisherError) Error() string {
	return s.Msg
}

// Defines the structure of a Metric message
type Metric struct {
	Timestamp float64                `json:"timestamp"`
	Metric    string                 `json:"metric"`
	Value     float64                `json:"value"`
	Tags      map[string]interface{} `json:"tags"`
	Error     bool                   `json:"error"`
}

func (m *Metric) HasTracer() bool {
	if _, ok := m.Tags["mtrace"]; ok {
		return true
	}
	return false
}

func (m *Metric) TracerMessage(msg string) {
	plog.WithFields(logrus.Fields{
		"mtrace":   m.Tags["mtrace"],
		"metric":   m.Metric,
		"timetamp": m.Timestamp,
		"value":    m.Value,
		"tags":     m.Tags,
	}).Info(msg)
}

//UnmarshalJSON supports string and non-string encoded Metric Values
func (m *Metric) UnmarshalJSON(data []byte) (err error) {
	fieldMap := map[string]interface{}{}
	err = json.Unmarshal(data, &fieldMap)
	if err != nil {
		return err
	}

	//convert timestamp
	if v, ok := fieldMap["timestamp"]; ok {
		if m.Timestamp, ok = v.(float64); ok {
		} else if f, ok := v.(float32); ok {
			m.Timestamp = float64(f)
		} else {
			return fmt.Errorf("Illegal metric timestamp: %s", v)
		}
	}

	//convert metric
	if v, ok := fieldMap["metric"]; ok {
		if m.Metric, ok = v.(string); !ok || m.Metric == "" {
			return fmt.Errorf("Illegal metric name: %v", v)
		}
	}

	//convert value, value may or may not be string encoded
	if v, ok := fieldMap["value"]; ok {
		if m.Value, ok = v.(float64); ok {
			//isn't that nice.. it's already a float64
		} else if f, ok := v.(float32); ok {
			//convert the float32 into a float64
			m.Value = float64(f)
		} else if s, ok := v.(string); ok {
			//support string encoded values
			m.Value, err = strconv.ParseFloat(s, 64)
			if err != nil {
				return fmt.Errorf("Illegal metric value: %s", s)
			}
		} else {
			return fmt.Errorf("Illegal metric value: %s", v)
		}
	}

	//convert tags
	if v, ok := fieldMap["tags"]; ok {
		if v == nil {
		} else if m.Tags, ok = v.(map[string]interface{}); ok {
		} else {
			return fmt.Errorf("Illegal metric tags: %s", v)
		}
	}
	return err
}

func (m Metric) Equal(that Metric) bool {
	if math.Abs(m.Timestamp-that.Timestamp) > 0.000000001 {
		return false
	}

	if math.Abs(m.Value-that.Value) > 0.000000001 {
		return false
	}

	if m.Metric != that.Metric {
		return false
	}

	return reflect.DeepEqual(m.Tags, that.Tags)
}

// Structure of message forwarded via websocket
type MetricBatch struct {
	Control interface{} `json:"control"` // Should be nil
	Metrics []Metric    `json:"metrics"`
}

func (b MetricBatch) Tracer(msg string) {
	for _, m := range b.Metrics {
		if m.HasTracer() {
			m.TracerMessage(msg)
		}
	}
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
