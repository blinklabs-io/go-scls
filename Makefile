# Determine root directory
ROOT_DIR=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

# Gather all .go files for use in dependencies below
GO_FILES=$(shell find $(ROOT_DIR) -name '*.go')

# CLI module (nested) and version injection
CLI_DIR=cmd/scls
CLI_MODULE=github.com/blinklabs-io/go-scls/cmd/scls
VERSION=$(shell git describe --tags --exact-match 2>/dev/null)
COMMIT=$(shell git rev-parse --short HEAD)
GO_LDFLAGS=-ldflags "-X '$(CLI_MODULE)/internal/version.Version=$(VERSION)' -X '$(CLI_MODULE)/internal/version.CommitHash=$(COMMIT)'"

.PHONY: build install clean mod-tidy test nilaway format golines

mod-tidy:
	# Needed to fetch new dependencies and add them to go.mod
	go mod tidy
	cd $(CLI_DIR) && go mod tidy

format:
	go fmt ./...
	gofmt -s -w $(GO_FILES)

golines:
	golines -w --ignore-generated --chain-split-dots --max-len=80 --reformat-tags .

test: mod-tidy
	go test -v -race ./...
	cd $(CLI_DIR) && go test -v -race ./...

nilaway: mod-tidy
	go run go.uber.org/nilaway/cmd/nilaway@latest ./...

# Build the scls CLI (honors GOOS/GOARCH from the environment for cross-builds)
build:
	cd $(CLI_DIR) && CGO_ENABLED=0 go build $(GO_LDFLAGS) -o $(ROOT_DIR)/scls .

install: build
	install -m 0755 scls $(HOME)/.local/bin/scls

clean:
	rm -f scls
