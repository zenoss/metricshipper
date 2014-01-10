package metricshipper

import (
	"github.com/imdario/mergo"
	flags "github.com/zenoss/go-flags"
	"io"
	"io/ioutil"
	yaml "launchpad.net/goyaml"
	"os"
)

type ShipperConfig struct {
	ConfigFilePath  string  `long:"config" short:"c" description:"Path to configuration file"`
	RedisUrl        string  `long:"redis-url" description:"Redis URL to subscribe to" default:"redis://localhost:6379/0/metrics"`
	Readers         int     `long:"readers" description:"Maximum number of simultaneous readers from Redis" default:"2"`
	ConsumerUrl     string  `long:"consumer-url" description:"WebSocket URL of consumer to publish to" default:"ws://localhost:8080/ws/metrics/store"`
	Writers         int     `long:"writers" description:"Maximum number of simultaneous writers to the consumer" default:"1"`
	MaxBufferSize   int     `long:"max-buffer-size" description:"Maximum number of messages to keep in the internal buffer" default:"1024"`
	MaxBatchSize    int     `long:"max-batch-size" description:"Number of messages to send to the consumer in a single web socket call. This should be smaller than the buffer size." default:"128"`
	BatchTimeout    float64 `long:"batch-timeout-seconds" description:"Maximum time in seconds to wait for messages from the internal buffer to be ready before making a web socket call with current metrics." default:"1"`
	BackoffWindow   int     `long:"backoff-window-seconds" description:"Rolling time period in seconds to consider collision messages from the consumer." default:"60"`
	MaxBackoffSteps int     `long:"max-backoff-steps" description:"Maximum number of collisions to consider for exponential backoff." default:"16"`
	Username        string  `long:"username" description:"Username to use when connecting to the consumer"`
	Password        string  `long:"password" description:"Password to use when connecting to the consumer"`
	CPUs            int     `long:"num-cpus" description:"Number of CPUs to use. Defaults to number of logical CPUs available."`
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

	// Replace zero-value entries in command-line opts with config file values
	mergo.Merge(commandlineopts, *cfgfileopts)

	// Replace any remaining zero-value entries with defaults
	mergo.Merge(commandlineopts, *defaultopts)

	return commandlineopts, nil
}