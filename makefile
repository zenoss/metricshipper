PREFIX ?= $(ZENHOME)
PACKAGE=github.com/zenoss/metricshipper

#------------------------------------------------------------------------------#
# Build Repeatability with Godeps
#------------------------------------------------------------------------------#
# We manage go dependencies by 'godep restoring' from a checked-in list of go 
# packages at desired versions in:
#
#    ./Godeps
#
# This file is manually updated and thus requires some dev-vigilence if our 
# go imports change in name or version.
#
# Alternatively, one may run:
#
#    godep save -copy=false
#
# to generate the Godeps file based upon the src currently populated in 
# $GOPATH/src.  It may be useful to periodically audit the checked-in Godeps
# against the generated Godeps.
#------------------------------------------------------------------------------#
GODEP     = $(GOPATH)/bin/godep
Godeps    = Godeps
godep_SRC = github.com/tools/godep

# The presence of this file indicates that godep restore 
# has been run.  It will refresh when ./Godeps itself is updated.
Godeps_restored = .Godeps_restored

.PHONY: default
default: build

.PHONY: build
build: output/metricshipper

$(Godeps_restored): $(GODEP) $(Godeps)
	@echo "$(GODEP) restore" ;\
	$(GODEP) restore ;\
	rc=$$? ;\
	if [ $${rc} -ne 0 ] ; then \
		echo "ERROR: Failed $(GODEP) restore. [rc=$${rc}]" ;\
		echo "** Unable to restore your GOPATH to a baseline state." ;\
		echo "** Perhaps internet connectivity is down." ;\
		exit $${rc} ;\
	fi
	touch $@

# Download godep source to $GOPATH/src/.
$(GOSRC)/$(godep_SRC):
	go get $(godep_SRC)

# Make the installed godep primitive (under $GOPATH/bin/godep)
# dependent upon the directory that holds the godep source.
# If that directory is missing, then trigger the 'go get' of the
# source.
#
# This requires some make fu borrowed from:
#
#    https://lists.gnu.org/archive/html/help-gnu-utils/2007-08/msg00019.html
#
missing_godep_SRC = $(filter-out $(wildcard $(GOSRC)/$(godep_SRC)), $(GOSRC)/$(godep_SRC))
$(GODEP): | $(missing_godep_SRC)
	go install $(godep_SRC)

output/metricshipper: $(Godeps_restored)
	@go get
	@mkdir -p output
	@cd output && go build $(PACKAGE)

devinstall: output/metricshipper
	@install -m 755 output/metricshipper $(PREFIX)/bin/metricshipper

.PHONY: install
install: output/metricshipper
	@mkdir -p $(PREFIX)/etc/supervisor $(PREFIX)/bin $(PREFIX)/etc/metricshipper
	@install -m 755 output/metricshipper $(PREFIX)/bin/metricshipper
	@install -m 644 etc/metricshipper.yaml $(PREFIX)/etc/metricshipper/metricshipper.yaml
	@install -m 644 etc/metricshipper_supervisor.conf $(PREFIX)/etc/metricshipper/metricshipper_supervisor.conf
	@install -m 644 etc/supervisord.conf $(PREFIX)/etc/metricshipper/supervisord.conf
	@ln -s ../metricshipper/metricshipper_supervisor.conf $(PREFIX)/etc/supervisor || echo "Supervisor config already exists"

.PHONY: test
test:
	@go get
	@go test $(PACKAGE)/lib
	@go test $(PACKAGE)

.PHONY: docker dockertest dockerbuild clean
docker:
	@docker ps > /dev/null && echo "Docker ok"

.PHONY: dockertest
dockertest: docker
	@docker build -t zenoss/metricshipper-build .
	@docker run -v $${PWD}:/gosrc/src/$(PACKAGE) -t zenoss/metricshipper-build /bin/bash -c "service redis-server start && make clean test"

.PHONY: dockerbuild
dockerbuild: docker
	@docker build -t zenoss/metricshipper-build .
	@docker run -e UID=$$(id -u) -v $${PWD}:/gosrc/src/$(PACKAGE) -t zenoss/metricshipper-build /bin/bash -c "make clean build && chown -R $${UID}:$${UID} /gosrc/src/$(PACKAGE)/output"

.PHONY: clean_godeps
clean_godeps: | $(GODEP) $(Godeps)
	-$(GODEP) restore && go clean -r && go clean -i github.com/zenoss/metricshipper/... # this cleans all dependencies
	@if [ -f "$(Godeps_restored)" ];then \
		rm -f $(Godeps_restored) ;\
		echo "rm -f $(Godeps_restored)" ;\
	fi

.PHONY: clean
clean: clean_godeps
	@go clean
	@rm -rf output

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
