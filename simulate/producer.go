package main

import (
	"encoding/json"
	"fmt"
	"github.com/garyburd/redigo/redis"
	"github.com/zenoss/glog"
	flags "github.com/zenoss/go-flags"
	metricshipper "github.com/zenoss/metricshipper/lib"
)

// metric producer config parameters
type ProducerConfig struct {
	Total    int    `long:"total" short:"t" description:"How many metrics to queue" default:"6400"`
	Batch    int    `long:"batch" short:"b" description:"How many metrics to batch" default:"64"`
	RedisUri string `long:"redis-uri" short:"u" description:"Redis URI to write to" default:"redis://localhost:6379/0/metrics"`
}

// parse faux producer arguments
func parse_producer_args(args []string) (ProducerConfig, error) {
	config := ProducerConfig{}
	parser := flags.NewParser(&config, flags.Default)
	_, err := parser.Parse()
	return config, err
}

// open connection to redis uri
func create_connection(uri string) (client redis.Conn, config *metricshipper.RedisConnectionConfig, err error) {
	config, err = metricshipper.ParseRedisUri(uri)
	if err != nil {
		return nil, config, err
	}
	client, err = metricshipper.DialFunc(config)()
	return client, config, err
}

func generate(channel string, size int) []interface{} {
	metrics := make([]interface{}, size+1)
	metrics[0] = channel
	for i := 1; i < size+1; i += 1 {
		metric := Metric{}
		content, _ := json.Marshal(metric)
		metrics[i] = content
	}
	return metrics
}

// write metrics to channel
func produce(client redis.Conn, channel, controlChannel string, total, batch int) {
	for i := 0; i < total; i += batch {
		if err := client.Send("MULTI"); err != nil {
			panic(fmt.Sprintf("Error sending metrics command, MULTI: %s", err))
		}
		message := generate(channel, batch)
		if err := client.Send("LPUSH", message...); err != nil {
			panic(fmt.Sprintf("Error sending metrics command, LPUSH: %s", err))
		}
		if _, err := client.Do("EXEC"); err != nil {
			panic(fmt.Sprintf("Error sending command, EXEC: %s", err))
		}
	}
}

// faux producer
func producer(args []string) {
	config, err := parse_producer_args(args)
	if err != nil {
		return
	}

	client, redisConfig, err := create_connection(config.RedisUri)
	if err != nil {
		glog.Errorf("Failed opening redis uri: %s -> %s", config.RedisUri, err)
		return
	}
	produce(client, redisConfig.Channel, redisConfig.Channel+"-control", config.Total, config.Batch)
}
