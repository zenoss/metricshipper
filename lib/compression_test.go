package metricshipper

import "testing"

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
	uncompressed, err := decompress(compressed, changes)
	if err != nil {
		t.Fatalf("%+v", err)
	}
	if !uncompressed.Equal(metric) {
		t.Fatalf("Oh no the metrics don't match")
	}
}
