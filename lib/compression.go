package metricshipper

import (
	"fmt"
	"sync"
)

type TranslationMap struct {
	sync.Mutex
	last  int
	trans map[string]int
}

func (m *TranslationMap) Translate(s string) (repr int, isdelta bool) {
	if v, ok := m.trans[s]; ok {
		return v, false
	}
	m.Lock()
	defer m.Unlock()
	repr = m.last + 1
	m.last = repr
	m.trans[s] = repr
	isdelta = true
	return
}

type Mapper struct {
	trans TranslationMap
}

func NewMapper() *Mapper {
	return &Mapper{
		TranslationMap{
			trans: make(map[string]int),
		},
	}
}

func (mapper *Mapper) Compress(m *Metric) (*CompressedMetric, map[int]string) {
	var (
		isdelta bool
		c_key   int
		c_val   int
		c       = &CompressedMetric{}
	)
	deltas := make(map[int]string)
	// First copy the things that are already numeric
	c.Timestamp = m.Timestamp
	c.Value = m.Value
	// Now compress the strings
	if c.Metric, isdelta = mapper.trans.Translate(m.Metric); isdelta {
		deltas[c.Metric] = m.Metric
	}
	c.Tags = make(map[int]int)
	for k, v := range m.Tags {
		if c_key, isdelta = mapper.trans.Translate(k); isdelta {
			deltas[c_key] = k
		}
		string_val := v.(string)
		if c_val, isdelta = mapper.trans.Translate(string_val); isdelta {
			deltas[c_val] = string_val
		}
		c.Tags[c_key] = c_val
	}
	return c, deltas
}

// decompress() is just for testing
func decompress(c *CompressedMetric, dictionary map[int]string) (*Metric, error) {
	var (
		ok           bool
		c_key, c_val string
		m            = &Metric{}
		err          = fmt.Errorf("Metric has int not included in the translation dictionary")
	)
	// First copy the things that are already numeric
	m.Timestamp = c.Timestamp
	m.Value = c.Value
	// Now translate the ints back to strings
	if metric, ok := dictionary[c.Metric]; !ok {
		return nil, err
	} else {
		m.Metric = metric
	}
	m.Tags = make(map[string]interface{})
	for k, v := range c.Tags {
		if c_key, ok = dictionary[k]; !ok {
			return nil, err
		}
		if c_val, ok = dictionary[v]; !ok {
			return nil, err
		}
		m.Tags[c_key] = c_val
	}
	return m, nil
}