

default: dockerbuild

build: test
	@mkdir build
	@cd build && go build github.com/zenoss/metricshipper && chown -R $${UID}:$${UID} .

test: 
	@/etc/init.d/redis-server start
	@go get
	@go test github.com/zenoss/metricshipper/lib
	@go test github.com/zenoss/metricshipper

docker:
	@docker ps > /dev/null && echo "Docker ok"

dockerbuild: docker
	@docker build -t zenoss/metricshipper-build .
	@docker run -e UID=$$(id -u) -v $${PWD}:/gosrc/src/github.com/zenoss/metricshipper -t zenoss/metricshipper-build make clean build

clean:
	@go clean
	@rm -rf build
