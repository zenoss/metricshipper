PREFIX ?= $ZENHOME
PACKAGE=github.com/zenoss/metricshipper


default: build

.PHONY: default build install test docker dockertest dockerbuild clean

build: output/metricshipper

output/metricshipper:
	@go get
	@mkdir output
	@cd output && go build $(PACKAGE)

devinstall: output/metricshipper
	@install -m 755 output/metricshipper $(PREFIX)/bin/metricshipper

install: output/metricshipper
	@mkdir -p $(PREFIX)/etc/supervisor $(PREFIX)/bin $(PREFIX)/etc/metricshipper
	@install -m 755 output/metricshipper $(PREFIX)/bin/metricshipper
	@install -m 644 etc/metricshipper.yaml $(PREFIX)/etc/metricshipper/metricshipper.yaml
	@install -m 644 etc/metricshipper_supervisor.conf $(PREFIX)/etc/metricshipper/metricshipper_supervisor.conf
	@install -m 644 etc/supervisord.conf $(PREFIX)/etc/metricshipper/supervisord.conf
	@ln -s ../metricshipper/metricshipper_supervisor.conf $(PREFIX)/etc/supervisor || echo "Supervisor config already exists"

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
