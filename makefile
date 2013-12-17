PREFIX ?= /opt/zenoss
PACKAGE=github.com/zenoss/metricshipper


default: build

.PHONY: default build install test docker dockertest dockerbuild clean

build: output/metricshipper

output/metricshipper:
	@go get
	@mkdir output
	@cd output && go build $(PACKAGE)

install: output/metricshipper
	@install -m 755 output/metricshipper $(PREFIX)/bin

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
	@docker run -e UID=$$(id -u) -v $${PWD}:/gosrc/src/$(PACKAGE) -t zenoss/metricshipper-build /bin/bash -c "make clean build && chown -R $${UID}:$${UID} /gosrc/src/$(PACKAGE)/output"

scratchbuild:
	@export GOPATH=/tmp/metricshipper-build; \
		BUILDDIR=$$GOPATH/src/$(PACKAGE); \
		HERE=$$PWD; \
		mkdir -p $$BUILDDIR; \
		rsync -rad $$HERE/ $$BUILDDIR ; \
		cd $$BUILDDIR; \
		$(MAKE) clean build; \
		mkdir -p $$HERE/output; \
		mv $$BUILDDIR/output/* $$HERE/output

clean:
	@go clean
	@rm -rf output
