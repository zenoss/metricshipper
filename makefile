default: build

.PHONY: default build install test docker dockertest clean

build: output/metricshipper

output/metricshipper:
	@mkdir output
	@cd output && go build github.com/zenoss/metricshipper && chown -R $${UID}:$${UID} .

install: output/metricshipper
	@install -m 755 -g zenoss -o zenoss output/metricshipper $$ZENHOME/bin

test: 
	@/etc/init.d/redis-server start
	@go get
	@go test github.com/zenoss/metricshipper/lib
	@go test github.com/zenoss/metricshipper

docker:
	@docker ps > /dev/null && echo "Docker ok"

dockertest: docker
	@docker build -t zenoss/metricshipper-build .
	@docker run -e UID=$$(id -u) -v $${PWD}:/gosrc/src/github.com/zenoss/metricshipper -t zenoss/metricshipper-build make clean test

clean:
	@go clean
	@rm -rf output
