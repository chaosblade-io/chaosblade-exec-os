.PHONY: build clean test build_all linux_amd64 darwin_amd64 linux_arm64 darwin_arm64 help version_tool test_version_injection

BLADE_SRC_ROOT := $(shell pwd)

# Get main version number (remove v prefix and Git description info)
MAIN_VERSION := $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//' | sed 's/-[0-9]*-[a-z0-9]*.*//' || echo "1.7.4")
BLADE_VERSION ?= $(MAIN_VERSION)

# Git information (for version injection)
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
# Version injection ldflags at build time (using complete Git information)
VERSION_LDFLAGS := -X "github.com/chaosblade-io/chaosblade-exec-os/version.BladeVersion=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")" \
                   -X "github.com/chaosblade-io/chaosblade-exec-os/version.GitCommit=$(GIT_COMMIT)" \
                   -X "github.com/chaosblade-io/chaosblade-exec-os/version.BuildTime=$(BUILD_TIME)"
GO_FLAGS := -ldflags="-s -w $(VERSION_LDFLAGS)"

PLATFORMS := linux_amd64 darwin_amd64 linux_arm64 darwin_arm64

# Get current platform information
CURRENT_OS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
CURRENT_ARCH := $(shell uname -m)
ifeq ($(CURRENT_ARCH),x86_64)
CURRENT_ARCH := amd64
else ifeq ($(CURRENT_ARCH),aarch64)
CURRENT_ARCH := arm64
endif
CURRENT_PLATFORM := $(CURRENT_OS)_$(CURRENT_ARCH)

# Define platform-specific directory name
define get_platform_dir_name
$(BUILD_TARGET)/chaosblade-$(BLADE_VERSION)-$(1)
endef

# Define platform-specific binary directory
define get_platform_bin_dir
$(call get_platform_dir_name,$(1))/bin
endef

# Define platform-specific YAML directory
define get_platform_yaml_dir
$(call get_platform_dir_name,$(1))/yaml
endef

# Build for specified platform
define build_for_platform
	@echo "==> Building for $(1) (version: $(BLADE_VERSION))"
	@mkdir -p $(call get_platform_bin_dir,$(1)) $(call get_platform_yaml_dir,$(1))
	@GOOS=$(word 1,$(subst _, ,$(1))) GOARCH=$(word 2,$(subst _, ,$(1))) \
	CGO_ENABLED=0 $(GO) build $(GO_FLAGS) -o $(call get_platform_bin_dir,$(1))/chaos_os main.go
	@cp extra/strace $(call get_platform_bin_dir,$(1))/ 2>/dev/null || true
	@GOOS=$(CURRENT_OS) GOARCH=$(CURRENT_ARCH) $(GO) run build/spec.go $(call get_platform_yaml_dir,$(1))/$(OS_YAML_FILE_NAME)
	@echo "✓ Build completed for $(1)"
	@echo "  Binary: $(call get_platform_bin_dir,$(1))/chaos_os"
	@echo "  Version: $(BLADE_VERSION) (commit: $(GIT_COMMIT_SHORT))"
endef

# Build current platform version
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
	@CGO_ENABLED=0 $(GO) build $(GO_FLAGS) -o $(call get_platform_bin_dir,$(CURRENT_PLATFORM))/chaos_os main.go
	@cp extra/strace $(call get_platform_bin_dir,$(CURRENT_PLATFORM))/ 2>/dev/null || true
	@$(GO) run build/spec.go $(call get_platform_yaml_dir,$(CURRENT_PLATFORM))/$(OS_YAML_FILE_NAME)
	@echo "✓ Build completed for $(CURRENT_PLATFORM)"
	@echo "  Binary: $(call get_platform_bin_dir,$(CURRENT_PLATFORM))/chaos_os"
	@echo "  Version: $(BLADE_VERSION) (commit: $(GIT_COMMIT_SHORT))"

# Build for specified platform (clean and rebuild)
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

# Build all platform versions
build_all: clean linux_amd64 darwin_amd64 linux_arm64 darwin_arm64

# Build version tool
version_tool: cmd/version/main.go
	@echo "==> Building version tool..."
	@mkdir -p bin
	@CGO_ENABLED=0 $(GO) build $(GO_FLAGS) -o bin/version cmd/version/main.go
	@echo "✓ Version tool built: bin/version"

# Test version injection
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

# Display version information
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

.PHONY: format
format:
	@echo "Running goimports and gofumpt to format Go code..."
	@./hack/update-imports.sh
	@./hack/update-gofmt.sh

.PHONY: verify
verify:
	@echo "Verifying Go code formatting and import order..."
	@./hack/verify-gofmt.sh
	@./hack/verify-imports.sh

.PHONY: help
help:
	@echo "Available commands:"
	@echo "  make build           # Build current platform version ($(CURRENT_PLATFORM))"
	@echo "  make linux_amd64     # Build linux_amd64 version"
	@echo "  make darwin_amd64    # Build darwin_amd64 version"
	@echo "  make linux_arm64     # Build linux_arm64 version"
	@echo "  make darwin_arm64    # Build darwin_arm64 version"
	@echo "  make build_all       # Build all mainstream platform versions at once"
	@echo "  make test            # Run all unit tests"
	@echo "  make clean           # Clean build artifacts"
	@echo "  make version         # Display version information"
	@echo "  make version_tool    # Build version tool"
	@echo "  make test_version_injection # Test version injection"
	@echo "  make format          # Format Go code using goimports and gofumpt"
	@echo "  make verify          # Verify Go code formatting and import order"
	@echo "  make help            # Display help information"
	@echo ""
	@echo "Current Platform: $(CURRENT_PLATFORM)"
	@echo "Main Version: $(BLADE_VERSION)"
	@echo "Full Version: $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")"
	@echo "Git Commit: $(GIT_COMMIT_SHORT)"
