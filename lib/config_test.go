package metricshipper

import (
	"fmt"
	"github.com/go-yaml/yaml"
	"github.com/imdario/mergo"
	flags "github.com/zenoss/go-flags"
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
		"redisurl":      "http://testRedisUrl",
		"readers":       123,
		"consumerurl":   "http://testConsumerUrl",
		"writers":       321,
		"maxbuffersize": 1234,
		"maxbatchsize":  4321,
		"batchtimeout":  float64(1000),
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

func TestMergeOverrideNone(t *testing.T) {
	type MergeConfig struct {
		StringValue string  `long:"str" short:"s" description:"some string" default:"foo"`
		IntValue    int     `long:"int" short:"i" description:"some int" default:"2"`
		FloatValue  float64 `long:"float" short:"f" description:"some float" default:"1.1"`
	}

	defaultopts := &MergeConfig{}
	actualopts := &MergeConfig{StringValue: "bar", IntValue: 5, FloatValue: 6.0}
	expectedopts := *actualopts
	mergo.Merge(actualopts, *defaultopts)

	if !reflect.DeepEqual(expectedopts, *actualopts) {
		t.Errorf("default options should have been merged into empty destination\nexpected: %+v \n  actual: %+v", &expectedopts, actualopts)
	}
}

func TestMergeOverrideAllToEmpty(t *testing.T) {
	type MergeConfig struct {
		StringValue string  `long:"str" short:"s" description:"some string" default:"foo"`
		IntValue    int     `long:"int" short:"i" description:"some int" default:"2"`
		FloatValue  float64 `long:"float" short:"f" description:"some float" default:"1.1"`
	}

	// Parse the options with no arguments to get defaults
	defaultopts := &MergeConfig{}
	flags.ParseArgs(defaultopts, make([]string, 0))
	expectedopts := *defaultopts

	expectedString := "&{StringValue:foo IntValue:2 FloatValue:1.1}"
	if expectedString != fmt.Sprintf("%+v", defaultopts) {
		t.Errorf("expected values did not match actual\nexpected: %+v \n  actual: %+v", expectedString, defaultopts)
	}

	actualopts := &MergeConfig{}
	mergo.Merge(actualopts, *defaultopts)

	if !reflect.DeepEqual(expectedopts, *actualopts) {
		t.Errorf("all non-empty options should have been merged into empty destination\nexpected: %+v \n  actual: %+v", &expectedopts, actualopts)
	}
}

func TestMergeOverrideAll(t *testing.T) {
	type MergeConfig struct {
		StringValue string  `long:"str" short:"s" description:"some string" default:"foo"`
		IntValue    int     `long:"int" short:"i" description:"some int" default:"2"`
		FloatValue  float64 `long:"float" short:"f" description:"some float" default:"1.1"`
	}

	// Parse the options with no arguments to get defaults
	defaultopts := &MergeConfig{}
	flags.ParseArgs(defaultopts, make([]string, 0))
	expectedopts := *defaultopts

	expectedString := "&{StringValue:foo IntValue:2 FloatValue:1.1}"
	if expectedString != fmt.Sprintf("%+v", defaultopts) {
		t.Errorf("expected values did not match actual\nexpected: %+v \n  actual: %+v", expectedString, defaultopts)
	}

	actualopts := &MergeConfig{StringValue: "bar", IntValue: 5, FloatValue: 6.0}
	mergo.Merge(actualopts, *defaultopts)

	if !reflect.DeepEqual(expectedopts, *actualopts) {
		t.Errorf("all non-empty options should have been merged into empty destination\nexpected: %+v \n  actual: %+v", &expectedopts, actualopts)
	}
}
