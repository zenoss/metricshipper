package metricshipper

import (
	"bytes"
	"code.google.com/p/snappy-go/snappy"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sync"
)

// Binary encoding for metrics.

type dictionary struct {
	sync.Mutex
	last  int32
	trans map[string]int32
}

func (d *dictionary) get(s string) (int32, bool) {
	if val, ok := d.trans[s]; ok {
		return val, false
	}
	d.Lock()
	defer d.Unlock()
	d.last += 1
	d.trans[s] = d.last
	return d.last, true
}

func (batch *MetricBatch) MarshalBinary(d *dictionary, doSnappy bool) ([]byte, error) {
	var (
		metric_name int32
		tag_key     int32
		tag_val     int32
		change      bool
	)
	dict := make(map[string]string)
	buf := new(bytes.Buffer)
	// Write the API version
	binary.Write(buf, binary.BigEndian, int8(0))
	// Write the number of metrics
	binary.Write(buf, binary.BigEndian, int32(len(batch.Metrics)))
	for _, metric := range batch.Metrics {
		binary.Write(buf, binary.BigEndian, metric.Timestamp)
		if metric_name, change = d.get(metric.Metric); change {
			dict[fmt.Sprintf("%d", metric_name)] = metric.Metric
		}
		binary.Write(buf, binary.BigEndian, metric_name)
		binary.Write(buf, binary.BigEndian, metric.Value)
		// Write the number of tags
		binary.Write(buf, binary.BigEndian, int8(len(metric.Tags)))
		for k, v := range metric.Tags {
			if tag_key, change = d.get(k); change {
				dict[fmt.Sprintf("%d", tag_key)] = k
			}
			binary.Write(buf, binary.BigEndian, tag_key)
			// This is still an interface{} for some reason
			s := fmt.Sprintf("%v", v)
			if tag_val, change = d.get(s); change {
				dict[fmt.Sprintf("%d", tag_val)] = s
			}
			binary.Write(buf, binary.BigEndian, tag_val)
		}
	}
	// Finally, marshal the translation dictionary updates as json so we can
	// write it as the remainder of the message
	changes, err := json.Marshal(dict)
	if err != nil {
		return nil, err
	}
	buf.Write(changes)

	if doSnappy {
		result := []byte{}
		return snappy.Encode(result, buf.Bytes())
	}

	return buf.Bytes(), nil
}
