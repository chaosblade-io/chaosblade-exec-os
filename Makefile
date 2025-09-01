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

# 获取当前平台信息
CURRENT_OS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
CURRENT_ARCH := $(shell uname -m)
ifeq ($(CURRENT_ARCH),x86_64)
CURRENT_ARCH := amd64
else ifeq ($(CURRENT_ARCH),aarch64)
CURRENT_ARCH := arm64
endif
CURRENT_PLATFORM := $(CURRENT_OS)_$(CURRENT_ARCH)

# 定义平台特定的目录名
define get_platform_dir_name
$(BUILD_TARGET)/chaosblade-$(BLADE_VERSION)-$(1)
endef

# 定义平台特定的二进制目录
define get_platform_bin_dir
$(call get_platform_dir_name,$(1))/bin
endef

# 定义平台特定的YAML目录
define get_platform_yaml_dir
$(call get_platform_dir_name,$(1))/yaml
endef

# 为指定平台构建
define build_for_platform
	@echo "==> Building for $(1)"
	@mkdir -p $(call get_platform_bin_dir,$(1)) $(call get_platform_yaml_dir,$(1))
	@GOOS=$(word 1,$(subst _, ,$(1))) GOARCH=$(word 2,$(subst _, ,$(1))) \
	$(GO) build $(GO_FLAGS) -o $(call get_platform_bin_dir,$(1))/chaos_os main.go
	@cp extra/strace $(call get_platform_bin_dir,$(1))/ 2>/dev/null || true
	@$(GO) run build/spec.go $(call get_platform_yaml_dir,$(1))/$(OS_YAML_FILE_NAME)
endef

# 构建当前平台版本
build: pre_build_current build_current_platform

pre_build_current:
	@echo "==> Building for current platform: $(CURRENT_PLATFORM)"
	@rm -rf $(call get_platform_dir_name,$(CURRENT_PLATFORM))
	@mkdir -p $(call get_platform_bin_dir,$(CURRENT_PLATFORM)) $(call get_platform_yaml_dir,$(CURRENT_PLATFORM))

build_current_platform:
	@$(GO) build $(GO_FLAGS) -o $(call get_platform_bin_dir,$(CURRENT_PLATFORM))/chaos_os main.go
	@cp extra/strace $(call get_platform_bin_dir,$(CURRENT_PLATFORM))/ 2>/dev/null || true
	@$(GO) run build/spec.go $(call get_platform_yaml_dir,$(CURRENT_PLATFORM))/$(OS_YAML_FILE_NAME)

# 为指定平台构建（清理并重建）
linux_amd64:
	@rm -rf $(call get_platform_dir_name,linux_amd64)
	@$(call build_for_platform,linux_amd64)

darwin_amd64:
	@rm -rf $(call get_platform_dir_name,darwin_amd64)
	@$(call build_for_platform,darwin_amd64)

linux_arm64:
	@rm -rf $(call get_platform_dir_name,linux_arm64)
	@$(call build_for_platform,linux_arm64)

darwin_arm64:
	@rm -rf $(call get_platform_dir_name,darwin_arm64)
	@$(call build_for_platform,darwin_arm64)

# 构建所有平台版本
build_all: clean linux_amd64 darwin_amd64 linux_arm64 darwin_arm64

test:
	$(GO) test -race -coverprofile=coverage.txt -covermode=atomic ./...

clean:
	go clean ./...
	rm -rf $(BUILD_TARGET)

help:
	@echo "可用命令："
	@echo "  make build           # 编译当前平台版本 ($(CURRENT_PLATFORM))"
	@echo "  make linux_amd64     # 编译 linux_amd64 版本"
	@echo "  make darwin_amd64    # 编译 darwin_amd64 版本"
	@echo "  make linux_arm64     # 编译 linux_arm64 版本"
	@echo "  make darwin_arm64    # 编译 darwin_arm64 版本"
	@echo "  make build_all       # 一键编译所有主流平台版本"
	@echo "  make test            # 运行所有单元测试"
	@echo "  make clean           # 清理构建产物"
	@echo "  make help            # 显示帮助信息"
	@echo ""
	@echo "当前平台: $(CURRENT_PLATFORM)"
	@echo "版本: $(BLADE_VERSION)"
