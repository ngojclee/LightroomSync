VERSION ?= 2.0.0.0

.PHONY: all build agent ui test clean

all: build

build:
	pwsh -NoProfile -ExecutionPolicy Bypass -File scripts/build_windows.ps1 -Version "$(VERSION)"

agent:
	go build -ldflags "-X main.Version=$(VERSION)" -o build/bin/LightroomSyncAgent.exe ./cmd/agent

ui:
	go build -ldflags "-X main.Version=$(VERSION)" -o build/bin/LightroomSyncUI.exe ./cmd/ui

test:
	go test ./... -count=1

clean:
	pwsh -NoProfile -ExecutionPolicy Bypass -Command "if (Test-Path 'build/bin') { Remove-Item -LiteralPath 'build/bin' -Recurse -Force }"
