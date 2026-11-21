# SPDX-License-Identifier: Apache-2.0
# Cross-compile from Linux, macOS or WSL. On Windows use build.ps1.
VERSION ?= 1.3.0
LDFLAGS := -s -w -H windowsgui -X snippingtoolenhanced/internal/editor.Version=$(VERSION)

.PHONY: build test
build:
	@test -f go.sum || go mod tidy
	go run -tags genres tools/genres/main.go $(VERSION)
	for a in amd64 arm64; do GOOS=windows GOARCH=$$a go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$$a/SnippingToolEnhanced.exe ./cmd/snippingtoolenhanced || exit 1; done

test:
	@test -f go.sum || go mod tidy
	go test ./...
	python3 test/editor_test.py
