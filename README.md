metricshipper
========

Simulation
------------
metricshipper provides a simulator for testing the shipper. The simulator
requires a redis server running in the background.

1. Build
 make clean ; make
 cd simulate && godep restore && go build

2. Execute - Consumer
  ./simulate/simulate consumer

3. Execute - Shipper
  ./output/metricshipper --consumer-url ws://localhost:8443/ws/metrics/store

4. Execute - Producer
  ./simulate/simulate producer -t 512000 -b 128
