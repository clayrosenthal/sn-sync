SOURCE_FILES?=$$(go list ./... | grep -v /vendor/ | grep -v /mocks/)
TEST_PATTERN?=.
TEST_OPTIONS?=-race -v

clean:
	rm -rf ./dist

setup:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.55.2
	go install -v github.com/go-critic/go-critic/cmd/gocritic@latest

test:
	echo 'mode: atomic' > coverage.txt && go list ./... | grep -v testing.go | xargs -n1 -I{} sh -c 'go test -v -failfast -p 1 -parallel 1 -timeout=600s -covermode=atomic -coverprofile=coverage.tmp {} && tail -n +2 coverage.tmp >> coverage.txt' && rm coverage.tmp

cover: test
	go tool cover -html=coverage.txt
# don't open browser...	go tool cover -html=coverage.txt -o coverage.html

fmt:
	goimports -w .

lint:
	golangci-lint run --tests=false --enable-all --disable lll --disable interfacer --disable gochecknoglobals

ci: lint test

BUILD_TAG := $(shell git describe --tags 2>/dev/null)
BUILD_SHA := $(shell git rev-parse --short HEAD)
BUILD_DATE := $(shell date -u '+%Y/%m/%d:%H:%M:%S')

build:
	CGO_ENABLED=0 go build -ldflags '-s -w -X "main.version=[$(BUILD_TAG)-$(BUILD_SHA)] $(BUILD_DATE) UTC"' -o ".local_dist/sn-sync" cmd/sn-sync/main.go

build-all:
	GOOS=darwin  CGO_ENABLED=0 GOARCH=amd64 go build -ldflags '-s -w -X "main.version=[$(BUILD_TAG)-$(BUILD_SHA)] $(BUILD_DATE) UTC"' -o ".local_dist/sn-sync_darwin_amd64" cmd/sn-sync/main.go
	GOOS=linux   CGO_ENABLED=0 GOARCH=amd64 go build -ldflags '-s -w -X "main.version=[$(BUILD_TAG)-$(BUILD_SHA)] $(BUILD_DATE) UTC"' -o ".local_dist/sn-sync_linux_amd64" cmd/sn-sync/main.go
	GOOS=linux   CGO_ENABLED=0 GOARCH=arm   go build -ldflags '-s -w -X "main.version=[$(BUILD_TAG)-$(BUILD_SHA)] $(BUILD_DATE) UTC"' -o ".local_dist/sn-sync_linux_arm" cmd/sn-sync/main.go
	GOOS=linux   CGO_ENABLED=0 GOARCH=arm64 go build -ldflags '-s -w -X "main.version=[$(BUILD_TAG)-$(BUILD_SHA)] $(BUILD_DATE) UTC"' -o ".local_dist/sn-sync_linux_arm64" cmd/sn-sync/main.go
	GOOS=netbsd  CGO_ENABLED=0 GOARCH=amd64 go build -ldflags '-s -w -X "main.version=[$(BUILD_TAG)-$(BUILD_SHA)] $(BUILD_DATE) UTC"' -o ".local_dist/sn-sync_netbsd_amd64" cmd/sn-sync/main.go
	GOOS=openbsd CGO_ENABLED=0 GOARCH=amd64 go build -ldflags '-s -w -X "main.version=[$(BUILD_TAG)-$(BUILD_SHA)] $(BUILD_DATE) UTC"' -o ".local_dist/sn-sync_openbsd_amd64" cmd/sn-sync/main.go
	GOOS=freebsd CGO_ENABLED=0 GOARCH=amd64 go build -ldflags '-s -w -X "main.version=[$(BUILD_TAG)-$(BUILD_SHA)] $(BUILD_DATE) UTC"' -o ".local_dist/sn-sync_freebsd_amd64" cmd/sn-sync/main.go

build-linux:
	GOOS=linux CGO_ENABLED=0 GOARCH=amd64 go build -ldflags '-s -w -X "main.version=[$(BUILD_TAG)-$(BUILD_SHA)] $(BUILD_DATE) UTC"' -o ".local_dist/sn-sync_linux_amd64" cmd/sn-sync/main.go

mac-install: build
	install .local_dist/sn-sync_darwin_all /usr/local/bin/sn-sync

linux-install: build-linux
	sudo install .local_dist/sn-sync_linux_amd64 /usr/local/bin/sn-sync

critic:
	gocritic check-project .

find-updates:
	go list -u -m -json all | go-mod-outdated -update -direct

release-snapshot:
	goreleaser build --snapshot --clean --skip post-hooks

release-test:
	goreleaser build --snapshot --clean --skip post-hooks --single-target

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := build

