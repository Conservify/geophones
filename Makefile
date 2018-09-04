GOARCH ?= amd64
GOOS ?= linux
GO ?= env GOOS=$(GOOS) GOARCH=$(GOARCH) go
BUILD ?= build
BUILDARCH ?= $(BUILD)/$(GOOS)-$(GOARCH)

all: $(BUILD) $(BUILD)/render-ascii go-all

go-all: go-deps
	GOOS=linux GOARCH=amd64 make go-binaries
	GOOS=linux GOARCH=arm make go-binaries
	GOOS=darwin GOARCH=amd64 make go-binaries

$(BUILD):
	mkdir -p $(BUILD)

$(BUILD)/render-ascii: rendering/*.cpp
	$(CXX) -g -std=c++11 -Wall -o $@ $^ -lm -lpthread -lncurses

go-binaries: $(BUILDARCH)/render-archives

$(BUILDARCH)/render-archives: rendering/*.go
	$(GO) build -o $@ $^

go-deps:
	go get -u golang.org/x/sys/...
	go get -u github.com/lucasb-eyer/go-colorful

clean:
	rm -rf $(BUILD)
