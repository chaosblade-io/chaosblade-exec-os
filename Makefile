.PHONY: build clean test build_all linux_amd64 darwin_amd64 linux_arm64 darwin_arm64 help

BLADE_SRC_ROOT := $(shell pwd)
BLADE_VERSION ?= 1.7.4

BUILD_TARGET := target
BUILD_TARGET_DIR_NAME := chaosblade-$(BLADE_VERSION)
BUILD_TARGET_PKG_DIR := $(BUILD_TARGET)/$(BUILD_TARGET_DIR_NAME)
BUILD_TARGET_BIN := $(BUILD_TARGET_PKG_DIR)/bin
BUILD_TARGET_YAML := $(BUILD_TARGET_PKG_DIR)/yaml

OS_YAML_FILE_NAME := chaosblade-os-spec-$(BLADE_VERSION).yaml
OS_YAML_FILE_PATH := $(BUILD_TARGET_YAML)/$(OS_YAML_FILE_NAME)

GO := go
GO_FLAGS := -ldflags="-s -w"

PLATFORMS := linux_amd64 darwin_amd64 linux_arm64 darwin_arm64

define build_for_platform
	@echo "==> Building for $(1)"
	@GOOS=$(word 1,$(subst _, ,$(1))) GOARCH=$(word 2,$(subst _, ,$(1))) \
	$(GO) build $(GO_FLAGS) -o $(BUILD_TARGET_BIN)/chaos_os_$(1) main.go
	@cp extra/strace $(BUILD_TARGET_BIN) || true
endef

build: pre_build build_yaml build_os

pre_build:
	rm -rf $(BUILD_TARGET_PKG_DIR)
	mkdir -p $(BUILD_TARGET_BIN) $(BUILD_TARGET_YAML)

build_yaml: build/spec.go
	$(GO) run $< $(OS_YAML_FILE_PATH)

build_os: main.go
	$(GO) build $(GO_FLAGS) -o $(BUILD_TARGET_BIN)/chaos_os main.go
	cp extra/strace $(BUILD_TARGET_BIN) || true

linux_amd64:
	$(call build_for_platform,linux_amd64)

darwin_amd64:
	$(call build_for_platform,darwin_amd64)

linux_arm64:
	$(call build_for_platform,linux_arm64)

darwin_arm64:
	$(call build_for_platform,darwin_arm64)

build_all: pre_build build_yaml linux_amd64 darwin_amd64 linux_arm64 darwin_arm64

test:
	$(GO) test -race -coverprofile=coverage.txt -covermode=atomic ./...

clean:
	go clean ./...
	rm -rf $(BUILD_TARGET)

help:
	@echo "可用命令："
	@echo "  make build           # 编译当前平台版本"
	@echo "  make linux_amd64     # 编译 linux_amd64 版本"
	@echo "  make darwin_amd64    # 编译 darwin_amd64 版本"
	@echo "  make linux_arm64     # 编译 linux_arm64 版本"
	@echo "  make darwin_arm64    # 编译 darwin_arm64 版本"
	@echo "  make build_all       # 一键编译所有主流平台版本"
	@echo "  make test            # 运行所有单元测试"
	@echo "  make clean           # 清理构建产物"
	@echo "  make help            # 显示帮助信息"
