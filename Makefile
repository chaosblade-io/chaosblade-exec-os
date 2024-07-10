.PHONY: build clean

BLADE_SRC_ROOT=$(shell pwd)

GO_ENV=CGO_ENABLED=1
GO_MODULE=GO111MODULE=on
GO=env $(GO_ENV) $(GO_MODULE) go
GO_FLAGS=-ldflags="-s -w"

UNAME := $(shell uname)

ifeq ($(BLADE_VERSION), )
	BLADE_VERSION=1.7.4
endif

BUILD_TARGET=target
BUILD_TARGET_DIR_NAME=chaosblade-$(BLADE_VERSION)
BUILD_TARGET_PKG_DIR=$(BUILD_TARGET)/chaosblade-$(BLADE_VERSION)
BUILD_TARGET_BIN=$(BUILD_TARGET_PKG_DIR)/bin
BUILD_TARGET_YAML=$(BUILD_TARGET_PKG_DIR)/yaml
BUILD_IMAGE_PATH=build/image/blade
# cache downloaded file
BUILD_TARGET_CACHE=$(BUILD_TARGET)/cache

OS_YAML_FILE_NAME=chaosblade-os-spec-$(BLADE_VERSION).yaml
OS_YAML_FILE_PATH=$(BUILD_TARGET_YAML)/$(OS_YAML_FILE_NAME)

ifeq ($(GOOS), linux)
	GO_FLAGS=-ldflags="-linkmode external -extldflags -static -s -w"
endif

# build os
build: pre_build build_yaml build_os

pre_build:
	rm -rf $(BUILD_TARGET_PKG_DIR) $(BUILD_TARGET_PKG_FILE_PATH)
	mkdir -p $(BUILD_TARGET_BIN) $(BUILD_TARGET_YAML)

build_yaml: build/spec.go
	$(GO) run $< $(OS_YAML_FILE_PATH)

build_os: main.go
	$(GO) build $(GO_FLAGS) -o $(BUILD_TARGET_BIN)/chaos_os $<
	cp extra/strace $(BUILD_TARGET_BIN)

cp_strace:
	cp extra/strace $(BUILD_TARGET_BIN)/

# build chaosblade linux version by docker image
build_linux:
	docker build -f build/image/musl/Dockerfile -t chaosblade-os-build-musl:latest build/image/musl
	docker run --rm \
		-v $(shell echo -n ${GOPATH}):/go \
		-v $(BLADE_SRC_ROOT):/chaosblade-exec-os \
		-w /chaosblade-exec-os \
		chaosblade-os-build-musl:latest

# test
test:
	go test -race -coverprofile=coverage.txt -covermode=atomic ./...
# clean all build result
clean:
	go clean ./...
	rm -rf $(BUILD_TARGET)
	rm -rf $(BUILD_IMAGE_PATH)/$(BUILD_TARGET_DIR_NAME)
