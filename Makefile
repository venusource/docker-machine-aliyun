# Support go1.5 vendoring (let us avoid messing with GOPATH or using godep)
export GO15VENDOREXPERIMENT = 1

GODEP_BIN := $(GOPATH)/bin/godep
GODEP := $(shell [ -x $(GODEP_BIN) ] && echo $(GODEP_BIN) || echo '')

default: build

bin/docker-machine-driver-aliyun:
	go build -i -o ./bin/docker-machine-driver-aliyun ./bin

build: clean bin/docker-machine-driver-aliyun

clean:
	$(RM) bin/docker-machine-driver-aliyun

install: bin/docker-machine-driver-aliyun
	cp -f ./bin/docker-machine-driver-aliyun $(GOPATH)/bin/

dep-save:
	$(if $(GODEP), , \
		$(error Please install godep: go get github.com/tools/godep))
	$(GODEP) save $(shell go list ./... | grep -v vendor/)

dep-restore:
	$(if $(GODEP), , \
		$(error Please install godep: go get github.com/tools/godep))
	$(GODEP) restore -v

.PHONY: clean build dep-save dep-restore install