package metricshipper

import (
	"fmt"
	"testing"
)

func TestCompression(t *testing.T) {
	mapper := NewMapper()
	metric := Metric{
		Timestamp: 0,
		Metric:    "la",
		Value:     15.25,
		Tags: map[string]interface{}{
			"tenant_id": "XXX",
		},
	}
	compressed, changes := mapper.Compress(&metric)
	fmt.Printf("%+v %+v", compressed, changes)
}
