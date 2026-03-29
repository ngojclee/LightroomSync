VERSION ?= 2.0.0.0

.PHONY: all agent ui test clean

all: agent ui

agent:
	go build -ldflags "-X main.Version=$(VERSION)" -o build/bin/LightroomSyncAgent.exe ./cmd/agent

ui:
	go build -ldflags "-X main.Version=$(VERSION)" -o build/bin/LightroomSyncUI.exe ./cmd/ui

test:
	go test ./internal/... -v

clean:
	rm -rf build/bin/
