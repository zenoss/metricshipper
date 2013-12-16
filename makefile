PACKAGE=github.com/zenoss/metricshipper


default: build

.PHONY: default build install test docker dockertest dockerbuild clean

build: output/metricshipper

output/metricshipper:
	@mkdir output
	@cd output && go build $(PACKAGE) && chown -R $${UID}:$${UID} .

install: output/metricshipper
	@install -m 755 output/metricshipper $$ZENHOME/bin

test: 
	@go get
	@go test $(PACKAGE)/lib
	@go test $(PACKAGE)

docker:
	@docker ps > /dev/null && echo "Docker ok"

dockertest: docker
	@docker build -t zenoss/metricshipper-build .
	@docker run -v $${PWD}:/gosrc/src/$(PACKAGE) -t zenoss/metricshipper-build /bin/bash -c "service redis-server start && make clean test"

dockerbuild: docker
	@docker build -t zenoss/metricshipper-build .
	@docker run -e UID=$$(id -u) -v $${PWD}:/gosrc/src/$(PACKAGE) -t zenoss/metricshipper-build make clean build

clean:
	@go clean
	@rm -rf output
