package metricshipper

import (
	"strings"
	"testing"
)

type MetricFromJSONTestCase struct {
	Input    string
	Error    string
	Expected Metric
}

func TestMetricFromJSONParsesStringValue(t *testing.T) {
	MetricFromJSONTestCases := []MetricFromJSONTestCase{
		//timestamp tests
		MetricFromJSONTestCase{"{ \"timestamp\":\"1.0\"}", "Illegal metric timestamp:", Metric{}},
		MetricFromJSONTestCase{"{ \"timestamp\":\"\"}", "Illegal metric timestamp:", Metric{}},
		MetricFromJSONTestCase{"{ \"timestamp\":[]}", "Illegal metric timestamp:", Metric{}},
		MetricFromJSONTestCase{"{ \"timestamp\":123456.00000}", "", Metric{Timestamp: 123456.0}},

		//metric tests
		MetricFromJSONTestCase{"{ \"metric\":1}", "Illegal metric name:", Metric{}},
		MetricFromJSONTestCase{"{ \"metric\":[]}", "Illegal metric name:", Metric{}},
		MetricFromJSONTestCase{"{ \"metric\":\"\"}", "Illegal metric name:", Metric{}},
		MetricFromJSONTestCase{"{ \"metric\":\"1\"}", "", Metric{Metric: "1"}},

		//value tests
		MetricFromJSONTestCase{"{ \"value\":\"\"}", "Illegal metric value:", Metric{}},
		MetricFromJSONTestCase{"{ \"value\":\"a\"}", "Illegal metric value:", Metric{}},
		MetricFromJSONTestCase{"{ \"value\":[]}", "Illegal metric value:", Metric{}},
		MetricFromJSONTestCase{"{ \"value\":{}}", "Illegal metric value:", Metric{}},
		MetricFromJSONTestCase{"{ \"value\":1}", "", Metric{Value: 1.0}},
		MetricFromJSONTestCase{"{ \"value\":1.0}", "", Metric{Value: 1.0}},
		MetricFromJSONTestCase{"{ \"value\":\"1\"}", "", Metric{Value: 1.0}},

		//tags tests
		MetricFromJSONTestCase{"{ \"tags\":1}", "Illegal metric tags:", Metric{}},
		MetricFromJSONTestCase{"{ \"tags\":\"\"}", "Illegal metric tags:", Metric{}},
		MetricFromJSONTestCase{"{ \"tags\":\"a\"}", "Illegal metric tags:", Metric{}},
		MetricFromJSONTestCase{"{ \"tags\":[]}", "Illegal metric tags:", Metric{}},

		MetricFromJSONTestCase{"{ \"tags\":null}", "", Metric{}},
		MetricFromJSONTestCase{"{ \"tags\":{}}", "", Metric{Tags: map[string]interface{}{}}},
		MetricFromJSONTestCase{"{ \"tags\":{\"1\":\"1\",\"2\":\"2\"}}", "", Metric{Tags: map[string]interface{}{"1": "1", "2": "2"}}},

		//successful test
		MetricFromJSONTestCase{
			`{
        "timestamp": 0,
        "metric": "la",
        "value": 15.25,
        "tags": {
          "tenant_id": "XXX"
        }
       }`,
			"",
			Metric{
				Timestamp: 0,
				Metric:    "la",
				Value:     15.25,
				Tags: map[string]interface{}{
					"tenant_id": "XXX",
				},
			},
		},
	}

	for _, testcase := range MetricFromJSONTestCases {
		actual, err := MetricFromJSON([]byte(testcase.Input))
		if err == nil && testcase.Error == "" && !actual.Equal(testcase.Expected) {
			t.Errorf(" expected (%+v) != actual (%+v)", testcase.Expected, *actual)
		} else if err == nil && testcase.Error != "" {
			t.Errorf(" expected error prefix \"%s\", got nil w/actual=%+v", testcase.Error, *actual)
		} else if err != nil && testcase.Error == "" {
			t.Errorf(" unexpected error \"%s\", expected %+v", err, testcase.Expected)
		} else if err != nil && !strings.HasPrefix(err.Error(), testcase.Error) {
			t.Errorf(" expected error prefix \"%s\", got %s", testcase.Error, err)
		}
	}
}
