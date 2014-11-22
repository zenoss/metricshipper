package metricshipper

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"sync"
)

// Binary encoding for metrics.

type dictionary struct {
	sync.Mutex
	last  uint32
	trans map[string]uint32
}

func (d *dictionary) get(s string) (uint32, bool) {
	if val, ok := d.trans[s]; ok {
		return val, false
	}
	d.Lock()
	defer d.Unlock()
	d.last += 1
	d.trans[s] = d.last
	return d.last, true
}

func (batch *MetricBatch) MarshalBinary(d *dictionary) ([]byte, error) {
	var (
		metric_name uint32
		tag_key     uint32
		tag_val     uint32
		change      bool
	)
	dict := make(map[uint32]string)
	buf := new(bytes.Buffer)
	// Write the number of metrics (this could probably be 8-bit)
	binary.Write(buf, binary.BigEndian, uint16(len(batch.Metrics)))
	for _, metric := range batch.Metrics {
		binary.Write(buf, binary.BigEndian, metric.Timestamp)
		if metric_name, change = d.get(metric.Metric); change {
			dict[metric_name] = metric.Metric
		}
		binary.Write(buf, binary.BigEndian, metric_name)
		// Write the number of tags
		binary.Write(buf, binary.BigEndian, uint8(len(metric.Tags)))
		for k, v := range metric.Tags {
			if tag_key, change = d.get(k); change {
				dict[tag_key] = k
			}
			binary.Write(buf, binary.BigEndian, tag_key)
			// This is still an interface{} for some reason
			s := v.(string)
			if tag_val, change = d.get(s); change {
				dict[tag_val] = s
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
	return buf.Bytes(), nil
}