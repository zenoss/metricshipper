package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/zenoss/glog"
	flags "github.com/zenoss/go-flags"
)

// PerfProducerConfig contains metric producer config parameters
type PerfProducerConfig struct {
	Devices      int    `long:"devices" short:"d" description:"How many devices to simulate" default:"50000"`
	Prefix       string `String:"prefix" short:"p" description:"Device prefix" default:"device"`
	Components   int    `long:"components" short:"c" description:"How many components per device" default:"50"`
	CollInterval int    `long:"snmp-interval" short:"i" description:"What is the snmp collection interval (sec) " default:"300"`
	Duration     int    `long:"duration" short:"t" description:"How many long will the test run (min)" default:"15"`
	Batch        int    `long:"batch" short:"b" description:"How many metrics to batch" default:"64"`
	RedisUri     string `string:"redis-uri" short:"u" description:"Redis URI to write to" default:"redis://localhost:6379/0/metrics"`
}

// parse faux producer arguments
func parsePerfproducerArgs(args []string) (PerfProducerConfig, error) {
	config := PerfProducerConfig{}
	parser := flags.NewParser(&config, flags.Default)
	_, err := parser.Parse()
	return config, err
}

func perfgenerate(channel string, deviceIdx int, components int, prefix string) []interface{} {
	deviceMetrics := [4]string{"avgBusy5", "sysUpTime", "ciscoMemoryPoolFree", "ciscoMemoryPoolUsed"}
	compMetrics := [13]string{"ifHCInBroadcastPkts", "ifHCInMulticastPkts", "ifHCInOctets", "ifHCInUcastPkts", "ifHCOutBroadcastPkts", "ifHCOutMulticastPkts", "ifHCOutOctets", "ifHCOutUcastPkts", "ifInDiscards", "ifInErrors", "ifOperStatus", "ifOutDiscards", "ifOutErrors"}
	size := len(deviceMetrics) + len(compMetrics)*components + 1
	metrics := make([]interface{}, size)
	metrics[0] = channel

	i := 1
	gen := rand.New(rand.NewSource(time.Now().Unix()))
	// device devel metrics
	for d := 0; d < len(deviceMetrics); d++ {
		metric := Metric{}
		metric.Metric = deviceMetrics[d]
		metric.Timestamp = int(time.Now().Unix())
		metric.Value = gen.Float64()
		metric.Tags = make(map[string]interface{})
		metric.Tags["device"] = fmt.Sprintf("%s-%d", prefix, deviceIdx)
		metric.Tags["uuid"] = metric.Tags["device"]
		metric.Tags["datasource"] = metric.Metric
		metric.Tags["tenantid"] = "1yy47k0w99drg1htla4uvhdsg"
		content, _ := json.Marshal(metric)
		metrics[i] = content
		i++
	}
	// component level metrics
	for j := 0; j < components; j++ {
		for c := 0; c < len(compMetrics); c++ {
			metric := Metric{}
			metric.Metric = fmt.Sprintf("%s-%d", compMetrics[c], j)
			metric.Timestamp = int(time.Now().Unix())
			metric.Value = gen.Float64()
			metric.Tags = make(map[string]interface{})
			metric.Tags["device"] = fmt.Sprintf("%s-%d", prefix, deviceIdx)
			metric.Tags["uuid"] = fmt.Sprintf("component-%d-%d", deviceIdx, j)
			metric.Tags["datasource"] = metric.Metric
			metric.Tags["tenantid"] = "1yy47k0w99drg1htla4uvhdsg"
			content, _ := json.Marshal(metric)
			metrics[i] = content
			i++
		}
	}
	return metrics
}

// write metrics to channel
func perfproduce(client redis.Conn, channel, controlChannel string, totalDevices int, components int, interval int, prefix string) {
	starttime := int(time.Now().Unix())
	for i := 0; i < totalDevices; i++ {
		message := perfgenerate(channel, i, components, prefix)
		if _, err := client.Do("LPUSH", message...); err != nil {
			panic(fmt.Sprintf("Error pushing metrics, LPUSH: %s", err))
		}
		if i%1000 == 0 {
			fmt.Printf("Sent metrics for device #%d to redis.\n", i+1)
		}
	}
	fmt.Printf("Sent metrics for %d devices to redis.\n", totalDevices)
	endtime := int(time.Now().Unix())
	// duration in secs
	duration := endtime - starttime

	if duration < interval {
		fmt.Printf("sleep for %d seconds\n", interval-duration)
		sleepDuration := time.Duration(interval - duration)
		time.Sleep(sleepDuration * time.Second)
	}
	fmt.Println("Woke up")
}

// faux producer
func perfproducer(args []string) {
	config, err := parsePerfproducerArgs(args)
	if err != nil {
		return
	}

	client, redisConfig, err := create_connection(config.RedisUri)
	if err != nil {
		glog.Errorf("Failed opening redis uri: %s -> %s", config.RedisUri, err)
		fmt.Printf("%s\n", `Try using different uri:
	port=$(docker ps --no-trunc|awk '/redis-server/{print;exit}'|egrep -o '[[:digit:]]+->6379/tcp'|cut -f1 -d-);
	echo redis port to use is: $port
	./simulate perfproducer -u redis://$(uname -n):$port/0/metrics`)
		return
	}

	fmt.Printf("Number of Devices:%d  Components per Device:%d  Collection Interval(s):%d Producing Duration(min):%d\n", config.Devices, config.Components, config.CollInterval, config.Duration)
	iterations := config.Duration * 60 / config.CollInterval
	for i := 0; i < iterations; i++ {
		fmt.Printf("Iteration Number: %d\n", i+1)
		perfproduce(client, redisConfig.Channel, redisConfig.Channel+"-control", config.Devices, config.Components, config.CollInterval, config.Prefix)
	}
}
