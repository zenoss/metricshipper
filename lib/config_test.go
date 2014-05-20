package metricshipper

import (
	"fmt"
	yaml "gopkg.in/yaml.v1"
	"reflect"
	"strings"
	"testing"
)

func testConfigValues(config map[string]interface{}, t *testing.T) {
	result, _ := yaml.Marshal(config)
	reader := strings.NewReader(string(result))
	shipperConfig := &ShipperConfig{}
	LoadYAMLConfig(reader, shipperConfig)
	if shipperConfig == nil {
		t.Error("Unable to parse config")
	}
	r := reflect.ValueOf(shipperConfig).Elem()
	for k, v := range config {
		lowered := strings.ToLower(k)
		field := r.FieldByNameFunc(func(s string) bool {
			return strings.ToLower(s) == lowered
		})
		if v != field.Interface() {
			t.Error("Field " + k + " expected " + fmt.Sprint(v) + " but got " + fmt.Sprint(field.Interface()))
		}
	}
}

func TestParseEverything(t *testing.T) {
	config := map[string]interface{}{
		"redisurl":        "http://testRedisUrl",
		"readers":         123,
		"consumerurl":     "http://testConsumerUrl",
		"writers":         321,
		"maxbuffersize":   1234,
		"maxbatchsize":    4321,
		"batchtimeout":    float64(1000),
		"backoffwindow":   31415,
		"maxbackoffsteps": 1123,
	}
	testConfigValues(config, t)
}

func TestBadValue(t *testing.T) {
	config := map[string]interface{}{
		"maxbuffersize": "not a number",
	}
	result, _ := yaml.Marshal(config)
	reader := strings.NewReader(string(result))
	shipperConfig := &ShipperConfig{}
	LoadYAMLConfig(reader, shipperConfig)
	if shipperConfig.MaxBufferSize != 0 {
		t.Error("Max buffer size was parsed despite being invalid")
	}
}

func TestNoValue(t *testing.T) {
	config := map[string]interface{}{}
	result, _ := yaml.Marshal(config)
	reader := strings.NewReader(string(result))
	shipperConfig := &ShipperConfig{}
	LoadYAMLConfig(reader, shipperConfig)
	if shipperConfig.RedisUrl != "" {
		t.Error("Redis URL isn't empty")
	}
	if shipperConfig.Readers != 0 {
		t.Error("Readers isn't empty")
	}
	if shipperConfig.BatchTimeout != 0 {
		t.Error("Batch Timeout isn't empty")
	}
}
