VERSION ?= 2.0.0.0

.PHONY: all build installer e2e e2e-installer e2e-compare agent ui test clean

all: build

build:
	pwsh -NoProfile -ExecutionPolicy Bypass -File scripts/build_windows.ps1 -Version "$(VERSION)"

installer:
	pwsh -NoProfile -ExecutionPolicy Bypass -File scripts/build_installer.ps1 -Version "$(VERSION)"

e2e:
	pwsh -NoProfile -ExecutionPolicy Bypass -File scripts/e2e_windows_manual.ps1 -Mode all

e2e-installer:
	pwsh -NoProfile -ExecutionPolicy Bypass -File scripts/e2e_installer_regression.ps1

e2e-compare:
	pwsh -NoProfile -ExecutionPolicy Bypass -File scripts/e2e_two_machine_compare.ps1

agent:
	go build -ldflags "-X main.Version=$(VERSION)" -o build/bin/LightroomSyncAgent.exe ./cmd/agent

ui:
	go build -ldflags "-X main.Version=$(VERSION)" -o build/bin/LightroomSyncUI.exe ./cmd/ui

test:
	go test ./... -count=1

clean:
	pwsh -NoProfile -ExecutionPolicy Bypass -Command "if (Test-Path 'build/bin') { Remove-Item -LiteralPath 'build/bin' -Recurse -Force }"
