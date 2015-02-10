package metricshipper

import (
	"github.com/zenoss/glog"

	"bytes"
	"encoding/hex"
	"io/ioutil"
	"testing"
)

func TestMarshalBinary(t *testing.T) {
	buf := make([]Metric, 0)
	mb := &MetricBatch{
		Metrics: buf,
	}
	buf = append(buf, Metric{Timestamp: 1.0, Metric: "foo", Value: 2.0})
	buf = append(buf, Metric{Timestamp: 3.0, Metric: "bar", Value: 5.0})
	buf = append(buf, Metric{Timestamp: 7.0, Metric: "baz", Value: 11.0})
	mb.Metrics = buf

	dict := &dictionary{trans: make(map[string]int32)}
	actual, err := mb.MarshalBinary(dict, false)
	if err != nil {
		t.Fatalf("unable to marshal binary %s", err)
	}

	expfile := "encoding_test.expected"
	if false { // leave this as convenience to generate the expected file when things change
		if err := ioutil.WriteFile(expfile, actual, 0644); err != nil {
			t.Fatalf("unable to write to file %s %s", expfile, err)
		} else {
			glog.Infof("wrote marshalled metrics to file: %s\n%s", expfile, hex.Dump(actual))
		}
	}

	expected, err := ioutil.ReadFile(expfile)
	if err != nil {
		t.Fatalf("unable to read file %s %s", expfile, err)
	}

	if 0 != bytes.Compare(expected, actual) {
		t.Fatalf("expected does not match actual\n%s\n%s", hex.Dump(expected), hex.Dump(actual))
	}
}