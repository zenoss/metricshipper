package main

import (
	"github.com/zenoss/glog"
	"os"
	"strings"
)

//------------------------------------------------------------------------------
// Consumer Simulator
//------------------------------------------------------------------------------

func main() {
	if len(os.Args) <= 1 {
		glog.Errorf("Missing simulate argument (producer or consumer)")
		return
	}

	command := strings.ToLower(os.Args[1])
	if command == "producer" {
		glog.Infof("Simulating producer")
		producer(os.Args[1:])
	} else if command == "consumer" {
		glog.Infof("Simulating consumer")
		consumer(os.Args[1:])
	} else if command == "perfproducer" {
		glog.Infof("Simulating perfproducer")
		perfproducer(os.Args[1:])
	} else {
		glog.Errorf("Illegal simulate arguments %s, expected: (producer or consumer or perfproducer)", command)
	}
}
