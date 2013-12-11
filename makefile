

default: build

build: test
	@mkdir build
	@cd build && go build github.com/zenoss/metricd/metricd && chown -R $${UID}:$${UID} .

test: 
	@/etc/init.d/redis-server start
	@go get
	@go test

docker:
	@docker ps > /dev/null && echo "Docker ok"

dockerbuild: docker
	@docker build -t zenoss/metricd-build .
	@docker run -e UID=$$(id -u) -v $${PWD}:/gosrc/src/github.com/zenoss/metricd -t zenoss/metricd-build make clean build

clean:
	@go clean
	@rm -rf build
