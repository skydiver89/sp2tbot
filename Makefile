.PHONY: all build run clean

SHELL := /usr/bin/bash

# Allow override: make VOSK_DIR=/custom/path
VOSK_DIR ?= $(CURDIR)/vosk-linux-x86_64-0.3.45
BINARY ?= sp2tbot

# Export env like buildrun.sh
export LD_LIBRARY_PATH := $(VOSK_DIR)
export CGO_CPPFLAGS := -I$(VOSK_DIR)
export CGO_LDFLAGS := -L $(VOSK_DIR)

all: build

build:
	go build -o $(BINARY)

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY)


