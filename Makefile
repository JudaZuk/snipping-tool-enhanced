# SPDX-License-Identifier: Apache-2.0
# Cross-compile from Linux, macOS or WSL. On Windows use build.ps1.
# make build ARCH=amd64   builds x64 only
VERSION ?= 1.4.0
ARCH ?= amd64 arm64
LDFLAGS := -s -w -H windowsgui -X snippingtoolenhanced/internal/editor.Version=$(VERSION)

.PHONY: build test
build:
	@test -f go.sum || go mod tidy
	go run -tags genres tools/genres/main.go $(VERSION)
	for a in $(ARCH); do echo "Building $$a..."; GOOS=windows GOARCH=$$a go build -v -trimpath -ldflags "$(LDFLAGS)" -o dist/$$a/SnippingToolEnhanced.exe ./cmd/snippingtoolenhanced || exit 1; done

test:
	@test -f go.sum || go mod tidy
	go test ./...
	python3 test/editor_test.py
