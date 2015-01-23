package metricshipper

import (
	"github.com/go-yaml/yaml"
	"github.com/imdario/mergo"
	"github.com/zenoss/glog"
	flags "github.com/zenoss/go-flags"

	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

type ShipperConfig struct {
	ConfigFilePath         string  `long:"config" short:"c" description:"Path to configuration file"`
	RedisUrl               string  `long:"redis-url" description:"Redis URL to subscribe to" default:"redis://localhost:6379/0/metrics"`
	Readers                int     `long:"readers" description:"Maximum number of simultaneous readers from Redis" default:"2"`
	ConsumerUrl            string  `long:"consumer-url" description:"WebSocket URL of consumer to publish to" default:"ws://localhost:8080/ws/metrics/store"`
	Writers                int     `long:"writers" description:"Maximum number of simultaneous writers to the consumer" default:"1"`
	MaxBufferSize          int     `long:"max-buffer-size" description:"Maximum number of messages to keep in the internal buffer" default:"1024"`
	MaxBatchSize           int     `long:"max-batch-size" description:"Number of messages to send to the consumer in a single web socket call. This should be smaller than the buffer size." default:"64"`
	BatchTimeout           float64 `long:"batch-timeout-seconds" description:"Maximum time in seconds to wait for messages from the internal buffer to be ready before making a web socket call with current metrics." default:"1"`
	Encoding               string  `long:"encoding" description:"Encoding for metric publishing (valid values are 'json' or 'binary')" default:"binary"`
	BackoffWindow          int     `long:"backoff-window-seconds" description:"Rolling time period in seconds to consider collision messages from the consumer." default:"60"`
	MaxBackoffSteps        int     `long:"max-backoff-steps" description:"Maximum number of collisions to consider for exponential backoff." default:"1200"`
	MaxBackoffDelay        int     `long:"max-backoff-delay" description:"Maximum milliseconds per request to wait due to backoff (worst case)." default:"10000"`
	RetryConnectionTimeout int     `long:"retry-connection-timeout" description:"Sleep time between connection retry in seconds" default:"1"`
	MaxConnectionAge       int     `long:"max-connection-age" description:"Max lifespan of a websocket connection in seconds" default:"600"`
	Verbosity              int     `long:"verbosity" short:"v" description:"Set the glog logging verbosity" default:"0"`
	Username               string  `long:"username" description:"Username to use when connecting to the consumer"`
	Password               string  `long:"password" description:"Password to use when connecting to the consumer"`
	CPUs                   int     `long:"num-cpus" description:"Number of CPUs to use." default:"4"`
	StatsInterval          int     `long:"stats-interval" description:"Number of seconds between publishing stats" default:"30"`
}

func LoadYAMLConfig(reader io.Reader, cfg *ShipperConfig) error {
	bytes, err := ioutil.ReadAll(reader)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(bytes, cfg)
	if err != nil {
		return err
	}
	return nil
}

func ParseShipperConfig() (*ShipperConfig, error) {

	// Create the structs to be merged together later
	defaultopts := &ShipperConfig{}
	cfgfileopts := &ShipperConfig{}
	commandlineopts := &ShipperConfig{}

	// Parse command-line options with no defaults
	parser := flags.NewParser(commandlineopts, flags.Default|flags.IgnoreDefaults)
	if _, err := parser.Parse(); err != nil {
		return nil, err
	}

	// Now that we have the config file (if passed in), parse that
	if file, err := os.Open(commandlineopts.ConfigFilePath); err == nil {
		LoadYAMLConfig(file, cfgfileopts)
	}

	// Parse the options with no arguments to get defaults
	flags.ParseArgs(defaultopts, make([]string, 0))

	// Set runtimeopts to defaults
	runtimeopts := defaultopts

	// Replace runtimeopts with non-zero config file values
	mergo.Merge(runtimeopts, *cfgfileopts)

	// Replace runtimeopts with non-zero commandline values
	mergo.Merge(runtimeopts, *commandlineopts)

	glog.V(1).Infof("runtime metricshipper options: %+v", runtimeopts)

	// Validate encoding
	encoding := strings.ToLower(runtimeopts.Encoding)
	if encoding != "json" && encoding != "binary" {
		return nil, fmt.Errorf("Invalid encoding: %s", runtimeopts.Encoding)
	}
	glog.SetVerbosity(runtimeopts.Verbosity)

	return runtimeopts, nil
}
