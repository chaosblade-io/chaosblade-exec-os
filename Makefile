.PHONY: build clean

BLADE_SRC_ROOT=$(shell pwd)

GO_ENV=CGO_ENABLED=1
GO_MODULE=GO111MODULE=on
GO=env $(GO_ENV) $(GO_MODULE) go
GO_FLAGS=-ldflags="-s -w"

UNAME := $(shell uname)

ifeq ($(BLADE_VERSION), )
	BLADE_VERSION=0.8.0
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
build: pre_build build_yaml build_osbin

build_darwin: pre_build build_yaml build_osbin_darwin

pre_build:
	rm -rf $(BUILD_TARGET_PKG_DIR) $(BUILD_TARGET_PKG_FILE_PATH)
	mkdir -p $(BUILD_TARGET_BIN) $(BUILD_TARGET_YAML)

build_yaml: build/spec.go
	$(GO) run $< $(OS_YAML_FILE_PATH)

build_osbin: build_burncpu build_burnmem build_burnio build_killprocess build_stopprocess build_changedns build_tcnetwork build_dropnetwork build_filldisk build_occupynetwork build_appendfile build_chmodfile build_addfile build_deletefile build_movefile build_movefile build_kernel_delay build_kernel_error cp_strace

build_osbin_darwin: build_burncpu build_killprocess build_stopprocess build_changedns build_occupynetwork build_appendfile build_chmodfile build_addfile build_deletefile build_movefile

# build burn-cpu chaos tools
build_burncpu: exec/bin/burncpu/burncpu.go
	$(GO) build $(GO_FLAGS) -o $(BUILD_TARGET_BIN)/chaos_burncpu $<

# build burn-mem chaos tools
build_burnmem: exec/bin/burnmem/burnmem.go
	$(GO) build $(GO_FLAGS) -o $(BUILD_TARGET_BIN)/chaos_burnmem $<

# build burn-io chaos tools
build_burnio: exec/bin/burnio/burnio.go
	$(GO) build $(GO_FLAGS) -o $(BUILD_TARGET_BIN)/chaos_burnio $<

# build kill-process chaos tools
build_killprocess: exec/bin/killprocess/killprocess.go
	$(GO) build $(GO_FLAGS) -o $(BUILD_TARGET_BIN)/chaos_killprocess $<

# build stop-process chaos tools
build_stopprocess: exec/bin/stopprocess/stopprocess.go
	$(GO) build $(GO_FLAGS) -o $(BUILD_TARGET_BIN)/chaos_stopprocess $<

build_changedns: exec/bin/changedns/changedns.go
	$(GO) build $(GO_FLAGS) -o $(BUILD_TARGET_BIN)/chaos_changedns $<

build_tcnetwork: exec/bin/tcnetwork/tcnetwork.go
	$(GO) build $(GO_FLAGS) -o $(BUILD_TARGET_BIN)/chaos_tcnetwork $<

build_dropnetwork: exec/bin/dropnetwork/dropnetwork.go
	$(GO) build $(GO_FLAGS) -o $(BUILD_TARGET_BIN)/chaos_dropnetwork $<

build_filldisk: exec/bin/filldisk/filldisk.go
	$(GO) build $(GO_FLAGS) -o $(BUILD_TARGET_BIN)/chaos_filldisk $<

build_occupynetwork: exec/bin/occupynetwork/occupynetwork.go
	$(GO) build $(GO_FLAGS) -o $(BUILD_TARGET_BIN)/chaos_occupynetwork $<

build_appendfile: exec/bin/file/appendfile/appendfile.go
	$(GO) build $(GO_FLAGS) -o $(BUILD_TARGET_BIN)/chaos_appendfile $<

build_chmodfile: exec/bin/file/chmodfile/chmodfile.go
	$(GO) build $(GO_FLAGS) -o $(BUILD_TARGET_BIN)/chaos_chmodfile $<

build_addfile: exec/bin/file/addfile/addfile.go
	$(GO) build $(GO_FLAGS) -o $(BUILD_TARGET_BIN)/chaos_addfile $<

build_deletefile: exec/bin/file/deletefile/deletefile.go
	$(GO) build $(GO_FLAGS) -o $(BUILD_TARGET_BIN)/chaos_deletefile $<

build_movefile: exec/bin/file/movefile/movefile.go
	$(GO) build $(GO_FLAGS) -o $(BUILD_TARGET_BIN)/chaos_movefile $<

build_kernel_delay: exec/bin/kernel/delay/delay.go
	$(GO) build $(GO_FLAGS) -o $(BUILD_TARGET_BIN)/chaos_stracedelay $<

build_kernel_error: exec/bin/kernel/error/error.go
	$(GO) build $(GO_FLAGS) -o $(BUILD_TARGET_BIN)/chaos_straceerror $<

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
