package metricshipper

import (
	"github.com/zenoss/glog"

	"bytes"
	"encoding/hex"
	"io/ioutil"
	"testing"
)

func testMarshalBinary(t *testing.T, mb *MetricBatch, isSnappy bool, expectedFilename string) {
	dict := &dictionary{trans: make(map[string]int32)}
	actual, err := mb.MarshalBinary(dict, isSnappy)
	if err != nil {
		t.Fatalf("unable to marshal binary %s", err)
	}

	if false { // leave this as convenience to generate the expected file when things change
		if err := ioutil.WriteFile(expectedFilename, actual, 0644); err != nil {
			t.Fatalf("unable to write to file %s %s", expectedFilename, err)
		} else {
			glog.Infof("wrote marshalled metrics to file: %s\n%s", expectedFilename, hex.Dump(actual))
		}
	}

	expected, err := ioutil.ReadFile(expectedFilename)
	if err != nil {
		t.Fatalf("unable to read file %s %s", expectedFilename, err)
	}

	if 0 != bytes.Compare(expected, actual) {
		t.Fatalf("expected does not match actual\n%s\n%s", hex.Dump(expected), hex.Dump(actual))
	}
}

func TestMarshalBinary(t *testing.T) {
	buf := make([]Metric, 0)
	mb := &MetricBatch{
		Metrics: buf,
	}
	buf = append(buf, Metric{Timestamp: 1.0, Metric: "foo", Value: 2.0})
	buf = append(buf, Metric{Timestamp: 3.0, Metric: "bar", Value: 5.0})
	buf = append(buf, Metric{Timestamp: 7.0, Metric: "baz", Value: 11.0})
	mb.Metrics = buf

	testMarshalBinary(t, mb, false, "encoding_test.raw")
	testMarshalBinary(t, mb, true, "encoding_test.snappy")

}