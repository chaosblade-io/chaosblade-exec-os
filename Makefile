.PHONY: build clean

BLADE_SRC_ROOT=`pwd`

GO_ENV=CGO_ENABLED=1
GO_MODULE=GO111MODULE=on
GO=env $(GO_ENV) $(GO_MODULE) go

UNAME := $(shell uname)

ifeq ($(BLADE_VERSION), )
	BLADE_VERSION=0.4.0
endif

BUILD_TARGET=target
BUILD_TARGET_DIR_NAME=chaosblade-$(BLADE_VERSION)
BUILD_TARGET_PKG_DIR=$(BUILD_TARGET)/chaosblade-$(BLADE_VERSION)
BUILD_TARGET_BIN=$(BUILD_TARGET_PKG_DIR)/bin
BUILD_IMAGE_PATH=build/image/blade
# cache downloaded file
BUILD_TARGET_CACHE=$(BUILD_TARGET)/cache

OS_YAML_FILE_NAME=chaosblade-os-spec-$(BLADE_VERSION).yaml
OS_YAML_FILE_PATH=$(BUILD_TARGET_BIN)/$(OS_YAML_FILE_NAME)

ifeq ($(GOOS), linux)
	GO_FLAGS=-ldflags="-linkmode external -extldflags -static"
endif


# build os
build: pre_build build_yaml build_osbin

build_darwin: pre_build build_yaml build_osbin_darwin

pre_build:
	rm -rf $(BUILD_TARGET_PKG_DIR) $(BUILD_TARGET_PKG_FILE_PATH)
	mkdir -p $(BUILD_TARGET_BIN) $(BUILD_TARGET_LIB)

build_yaml: build/spec.go
	$(GO) run $< $(OS_YAML_FILE_PATH)

build_osbin: build_burncpu build_burnmem build_burnio build_killprocess build_stopprocess build_changedns build_tcnetwork build_dropnetwork build_filldisk

build_osbin_darwin: build_burncpu build_killprocess build_stopprocess build_changedns

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

# build chaosblade linux version by docker image
build_linux:
	docker build -f build/image/musl/Dockerfile -t chaosblade-build-musl:latest build/image/musl
	docker run --rm \
		-v $(shell echo -n ${GOPATH}):/go \
		-w /go/src/github.com/chaosblade-io/chaosblade-exec-os \
		chaosblade-build-musl:latest

# build chaosblade image for chaos
build_image: build_linux
	rm -rf $(BUILD_IMAGE_PATH)/$(BUILD_TARGET_DIR_NAME)

	cp -R $(BUILD_TARGET_PKG_DIR) $(BUILD_IMAGE_PATH)
	docker build -f $(BUILD_IMAGE_PATH)/Dockerfile \
		--build-arg BLADE_VERSION=$(BLADE_VERSION) \
		-t chaosblade-agent:$(BLADE_VERSION) \
		$(BUILD_IMAGE_PATH)

	rm -rf $(BUILD_IMAGE_PATH)/$(BUILD_TARGET_DIR_NAME)

# build docker image with multi-stage builds
docker_image: clean
	docker build -f ./Dockerfile \
		--build-arg BLADE_VERSION=$(BLADE_VERSION) \
		-t chaosblade:$(BLADE_VERSION) $(BLADE_SRC_ROOT)

# test
test:
	go test -race -coverprofile=coverage.txt -covermode=atomic ./...
# clean all build result
clean:
	go clean ./...
	rm -rf $(BUILD_TARGET)
	rm -rf $(BUILD_IMAGE_PATH)/$(BUILD_TARGET_DIR_NAME)
