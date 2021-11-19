.PHONY: test bench lint escape

MAKEFILE_PATH := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
GO := go
GOLANGCI_LINT_VERSION := v1.43.0
GOLANGCI_LINT := ./bin/golangci-lint-$(GOLANGCI_LINT_VERSION)

test: testdata
	$(GO) test -v -race

bench:
	$(GO) test -run XXX -bench . -benchmem

escape:
	$(GO) build -gcflags "-m -m" > escape.txt 2>&1

lint: bin/golangci-lint-$(GOLANGCI_LINT_VERSION)
	$(GOLANGCI_LINT) run

bin/golangci-lint-$(GOLANGCI_LINT_VERSION):
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(MAKEFILE_PATH)/bin $(GOLANGCI_LINT_VERSION)
	mv $(MAKEFILE_PATH)/bin/golangci-lint $(MAKEFILE_PATH)/bin/golangci-lint-$(GOLANGCI_LINT_VERSION)

testdata: testdata/suite testdata/small.xml

testdata/suite:
	wget -O suite.tar.gz https://www.w3.org/XML/Test/xmlts20130923.tar.gz
	mkdir $(MAKEFILE_PATH)/testdata/suite
	tar --strip-components=1 -xf suite.tar.gz -C $(MAKEFILE_PATH)testdata/suite
	rm suite.tar.gz

testdata/small.xml:
	wget -O small.gz http://aiweb.cs.washington.edu/research/projects/xmltk/xmldata/data/tpc-h/customer.xml.gz
	mkdir $(MAKEFILE_PATH)/testdata
	zcat small.gz > $(MAKEFILE_PATH)/testdata/small.xml
	rm small.gz
