.PHONY: build clean test build_all linux_amd64 darwin_amd64 linux_arm64 darwin_arm64 help version_tool test_version_injection

BLADE_SRC_ROOT := $(shell pwd)

# 获取主版本号（去掉 v 前缀和 Git 描述信息）
MAIN_VERSION := $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//' | sed 's/-[0-9]*-[a-z0-9]*.*//' || echo "1.7.4")
BLADE_VERSION ?= $(MAIN_VERSION)

# Git 信息（用于版本注入）
GIT_COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
GIT_COMMIT_SHORT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

BUILD_TARGET := target
BUILD_TARGET_DIR_NAME := chaosblade-$(BLADE_VERSION)
BUILD_TARGET_PKG_DIR := $(BUILD_TARGET)/$(BUILD_TARGET_DIR_NAME)
BUILD_TARGET_BIN := $(BUILD_TARGET_PKG_DIR)/bin
BUILD_TARGET_YAML := $(BUILD_TARGET_PKG_DIR)/yaml

OS_YAML_FILE_NAME := chaosblade-os-spec-$(BLADE_VERSION).yaml
OS_YAML_FILE_PATH := $(BUILD_TARGET_YAML)/$(OS_YAML_FILE_NAME)

GO := go
# 构建时版本注入的 ldflags（使用完整的 Git 信息）
VERSION_LDFLAGS := -X "github.com/chaosblade-io/chaosblade-exec-os/version.BladeVersion=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")" \
                   -X "github.com/chaosblade-io/chaosblade-exec-os/version.GitCommit=$(GIT_COMMIT)" \
                   -X "github.com/chaosblade-io/chaosblade-exec-os/version.BuildTime=$(BUILD_TIME)"
GO_FLAGS := -ldflags="-s -w $(VERSION_LDFLAGS)"

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
	@echo "==> Building for $(1) (version: $(BLADE_VERSION))"
	@mkdir -p $(call get_platform_bin_dir,$(1)) $(call get_platform_yaml_dir,$(1))
	@GOOS=$(word 1,$(subst _, ,$(1))) GOARCH=$(word 2,$(subst _, ,$(1))) \
	$(GO) build $(GO_FLAGS) -o $(call get_platform_bin_dir,$(1))/chaos_os main.go
	@cp extra/strace $(call get_platform_bin_dir,$(1))/ 2>/dev/null || true
	@$(GO) run build/spec.go $(call get_platform_yaml_dir,$(1))/$(OS_YAML_FILE_NAME)
	@echo "✓ Build completed for $(1)"
	@echo "  Binary: $(call get_platform_bin_dir,$(1))/chaos_os"
	@echo "  Version: $(BLADE_VERSION) (commit: $(GIT_COMMIT_SHORT))"
endef

# 构建当前平台版本
build: pre_build_current build_current_platform

pre_build_current:
	@echo "==> Building for current platform: $(CURRENT_PLATFORM)"
	@echo "  Main Version: $(BLADE_VERSION)"
	@echo "  Full Version: $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")"
	@echo "  Git Commit: $(GIT_COMMIT_SHORT)"
	@echo "  Build Time: $(BUILD_TIME)"
	@rm -rf $(call get_platform_dir_name,$(CURRENT_PLATFORM))
	@mkdir -p $(call get_platform_bin_dir,$(CURRENT_PLATFORM)) $(call get_platform_yaml_dir,$(CURRENT_PLATFORM))

build_current_platform:
	@$(GO) build $(GO_FLAGS) -o $(call get_platform_bin_dir,$(CURRENT_PLATFORM))/chaos_os main.go
	@cp extra/strace $(call get_platform_bin_dir,$(CURRENT_PLATFORM))/ 2>/dev/null || true
	@$(GO) run build/spec.go $(call get_platform_yaml_dir,$(CURRENT_PLATFORM))/$(OS_YAML_FILE_NAME)
	@echo "✓ Build completed for $(CURRENT_PLATFORM)"
	@echo "  Binary: $(call get_platform_bin_dir,$(CURRENT_PLATFORM))/chaos_os"
	@echo "  Version: $(BLADE_VERSION) (commit: $(GIT_COMMIT_SHORT))"

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

# 构建版本工具
version_tool: cmd/version/main.go
	@echo "==> Building version tool..."
	@mkdir -p bin
	@$(GO) build $(GO_FLAGS) -o bin/version cmd/version/main.go
	@echo "✓ Version tool built: bin/version"

# 测试版本注入
test_version_injection: version_tool
	@echo "==> Testing version injection..."
	@./bin/version
	@echo ""
	@echo "JSON format:"
	@./bin/version -json
	@echo ""
	@echo "Short version:"
	@./bin/version -short
	@echo ""
	@echo "Full version:"
	@./bin/version -full

# 显示版本信息
version:
	@echo "=== Version Information ==="
	@echo "Main Version: $(BLADE_VERSION)"
	@echo "Full Version: $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Git Commit Short: $(GIT_COMMIT_SHORT)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Current Platform: $(CURRENT_PLATFORM)"
	@echo "Go Version: $(shell go version)"

test:
	$(GO) test -race -coverprofile=coverage.txt -covermode=atomic ./...

clean:
	go clean ./...
	rm -rf $(BUILD_TARGET)
	rm -rf bin

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
	@echo "  make version         # 显示版本信息"
	@echo "  make version_tool    # 构建版本工具"
	@echo "  make test_version_injection # 测试版本注入"
	@echo "  make help            # 显示帮助信息"
	@echo ""
	@echo "当前平台: $(CURRENT_PLATFORM)"
	@echo "主版本: $(BLADE_VERSION)"
	@echo "完整版本: $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")"
	@echo "Git Commit: $(GIT_COMMIT_SHORT)"
