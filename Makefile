SHELL=/usr/bin/env bash

GO_BUILD_IMAGE?=golang:1.16.5
PG_IMAGE?=postgres:10
REDIS_IMAGE?=redis:6
VISOR_IMAGE_NAME?=filecoin/sentinel-visor
COMMIT := $(shell git rev-parse --short=8 HEAD)

# GITVERSION is the nearest tag plus number of commits and short form of most recent commit since the tag, if any
GITVERSION=$(shell git describe --always --tag --dirty)

unexport GOFLAGS

CLEAN:=
BINS:=

GOFLAGS:=

.PHONY: all
all: build

## FFI

FFI_PATH:=extern/filecoin-ffi/
FFI_DEPS:=.install-filcrypto
FFI_DEPS:=$(addprefix $(FFI_PATH),$(FFI_DEPS))

$(FFI_DEPS): build/.filecoin-install ;

build/.filecoin-install: $(FFI_PATH)
	$(MAKE) -C $(FFI_PATH) $(FFI_DEPS:$(FFI_PATH)%=%)
	@touch $@

MODULES+=$(FFI_PATH)
BUILD_DEPS+=build/.filecoin-install
CLEAN+=build/.filecoin-install

ffi-version-check:
	@[[ "$$(awk '/const Version/{print $$5}' extern/filecoin-ffi/version.go)" -eq 3 ]] || (echo "FFI version mismatch, update submodules"; exit 1)
BUILD_DEPS+=ffi-version-check

.PHONY: ffi-version-check


$(MODULES): build/.update-modules ;
# dummy file that marks the last time modules were updated
build/.update-modules:
	git submodule update --init --recursive
	touch $@

CLEAN+=build/.update-modules

# tools
toolspath:=support/tools

ldflags=-X=github.com/filecoin-project/sentinel-visor/version.GitVersion=$(GITVERSION)
ifneq ($(strip $(LDFLAGS)),)
	ldflags+=-extldflags=$(LDFLAGS)
endif
GOFLAGS+=-ldflags="$(ldflags)"

.PHONY: build
build: deps visor

.PHONY: deps
deps: $(BUILD_DEPS)

.PHONY: vector-setup
vector-setup:
	cd ./vector; ./fetch_vectors.sh
CLEAN+=./vector/data/*.json

# test starts dependencies and runs all tests
.PHONY: test
test: testfull

.PHONY: dockerup
dockerup:
	docker-compose up -d

.PHONY: dockerdown
dockerdown:
	docker-compose down

# testfull runs all tests
.PHONY: testfull
testfull: build
	docker-compose up -d
	sleep 2
	./visor migrate --latest
	-TZ= PGSSLMODE=disable go test ./... -v
	docker-compose down

# testshort runs tests that don't require external dependencies such as postgres or redis
.PHONY: testshort
testshort:
	go test -short ./... -v

.PHONY: visor
visor:
	rm -f visor
	go build $(GOFLAGS) -o visor -mod=readonly .
BINS+=visor

.PHONY: clean
clean:
	rm -rf $(CLEAN) $(BINS)

.PHONY: dist-clean
dist-clean:
	git clean -xdff
	git submodule deinit --all -f

.PHONY: test-coverage
test-coverage:
	VISOR_TEST_DB="postgres://postgres:password@localhost:5432/postgres?sslmode=disable" go test -coverprofile=coverage.out ./...

# tools

$(toolspath)/bin/golangci-lint: $(toolspath)/go.mod
	@mkdir -p $(dir $@)
	(cd $(toolspath); go build -tags tools -o $(@:$(toolspath)/%=%) github.com/golangci/golangci-lint/cmd/golangci-lint)

$(toolspath)/bin/gen: $(toolspath)/go.mod
	@mkdir -p $(dir $@)
	(cd $(toolspath); go build -tags tools -o $(@:$(toolspath)/%=%) github.com/filecoin-project/statediff/types/gen)


.PHONY: lint
lint: $(toolspath)/bin/golangci-lint
	$(toolspath)/bin/golangci-lint run ./...

.PHONY: actors-gen
actors-gen:
	go run ./chain/actors/agen
	go fmt ./...


.PHONY: types-gen
types-gen: $(toolspath)/bin/gen
	$(toolspath)/bin/gen ./tasks/messages/types
	go fmt ./tasks/messages/types/...

# dev-nets
2k: GOFLAGS+=-tags=2k
2k: build

calibnet: GOFLAGS+=-tags=calibnet
calibnet: build

nerpanet: GOFLAGS+=-tags=nerpanet
nerpanet: build

butterflynet: GOFLAGS+=-tags=butterflynet
butterflynet: build

interopnet: GOFLAGS+=-tags=interopnet
interopnet: build

# alias to match other network-specific targets
mainnet: build


# Dockerfiles

docker-files: Dockerfile Dockerfile.dev

Dockerfile:
	@echo "Writing ./Dockerfile..."
	@cat build/docker/header.tpl \
		build/docker/builder.tpl \
		build/docker/prod_entrypoint.tpl \
		> ./Dockerfile
CLEAN+=Dockerfile

Dockerfile.dev:
	@echo "Writing ./Dockerfile.dev..."
	@cat build/docker/header.tpl \
		build/docker/builder.tpl \
		build/docker/dev_entrypoint.tpl \
		> ./Dockerfile.dev
CLEAN+=Dockerfile.dev

# Docker images

# MAINNET
.PHONY: docker-mainnet
docker-mainnet: VISOR_DOCKER_FILE ?= Dockerfile
docker-mainnet: VISOR_NETWORK_TARGET ?= mainnet
docker-mainnet: VISOR_IMAGE_TAG ?= $(COMMIT)
docker-mainnet: docker-build-image-template

.PHONY: docker-mainnet-push
docker-mainnet-push: docker-mainnet docker-tag-and-push-template

.PHONY: docker-mainnet-dev
docker-mainnet-dev: VISOR_DOCKER_FILE ?= Dockerfile.dev
docker-mainnet-dev: VISOR_NETWORK_TARGET ?= mainnet
docker-mainnet-dev: VISOR_IMAGE_TAG ?= $(COMMIT)-dev
docker-mainnet-dev: docker-build-image-template

.PHONY: docker-mainnet-dev-push
docker-mainnet-dev-push: docker-mainnet-dev docker-tag-and-push-template

# CALIBNET
.PHONY: docker-calibnet
docker-calibnet: VISOR_DOCKER_FILE ?= Dockerfile
docker-calibnet: VISOR_NETWORK_TARGET ?= calibnet
docker-calibnet: VISOR_IMAGE_TAG ?= $(COMMIT)-calibnet
docker-calibnet: docker-build-image-template

.PHONY: docker-calibnet-push
docker-calibnet-push: docker-calibnet docker-tag-and-push-template

.PHONY: docker-calibnet-dev
docker-calibnet-dev: VISOR_DOCKER_FILE ?= Dockerfile.dev
docker-calibnet-dev: VISOR_NETWORK_TARGET ?= calibnet
docker-calibnet-dev: VISOR_IMAGE_TAG ?= $(COMMIT)-calibnet-dev
docker-calibnet-dev: docker-build-image-template

.PHONY: docker-calibnet-dev-push
docker-calibnet-dev-push: docker-calibnet-dev docker-tag-and-push-template

# INTEROPNET
.PHONY: docker-interopnet
docker-interopnet: VISOR_DOCKER_FILE ?= Dockerfile
docker-interopnet: VISOR_NETWORK_TARGET ?= interopnet
docker-interopnet: VISOR_IMAGE_TAG ?= $(COMMIT)-interopnet
docker-interopnet: docker-build-image-template

.PHONY: docker-interopnet-push
docker-interopnet-push: docker-interopnet docker-tag-and-push-template

.PHONY: docker-interopnet-dev
docker-interopnet-dev: VISOR_DOCKER_FILE ?= Dockerfile.dev
docker-interopnet-dev: VISOR_NETWORK_TARGET ?= interopnet
docker-interopnet-dev: VISOR_IMAGE_TAG ?= $(COMMIT)-interopnet-dev
docker-interopnet-dev: docker-build-image-template

.PHONY: docker-interopnet-dev-push
docker-interopnet-dev-push: docker-interopnet-dev docker-tag-and-push-template

# BUTTERFLYNET
.PHONY: docker-butterflynet
docker-butterflynet: VISOR_DOCKER_FILE ?= Dockerfile
docker-butterflynet: VISOR_NETWORK_TARGET ?= butterflynet
docker-butterflynet: VISOR_IMAGE_TAG ?= $(COMMIT)-butterflynet
docker-butterflynet: docker-build-image-template

.PHONY: docker-butterflynet-push
docker-butterflynet-push: docker-butterflynet docker-tag-and-push-template

.PHONY: docker-butterflynet-dev
docker-butterflynet-dev: VISOR_DOCKER_FILE ?= Dockerfile.dev
docker-butterflynet-dev: VISOR_NETWORK_TARGET ?= butterflynet
docker-butterflynet-dev: VISOR_IMAGE_TAG ?= $(COMMIT)-butterflynet-dev
docker-butterflynet-dev: docker-build-image-template

.PHONY: docker-butterflynet-dev-push
docker-butterflynet-dev-push: docker-butterflynet-dev docker-tag-and-push-template

# NERPANET
.PHONY: docker-nerpanet
docker-nerpanet: VISOR_DOCKER_FILE ?= Dockerfile
docker-nerpanet: VISOR_NETWORK_TARGET ?= nerpanet
docker-nerpanet: VISOR_IMAGE_TAG ?= $(COMMIT)-nerpanet
docker-nerpanet: docker-build-image-template

.PHONY: docker-nerpanet-push
docker-nerpanet-push: docker-nerpanet docker-tag-and-push-template

.PHONY: docker-nerpanet-dev
docker-nerpanet-dev: VISOR_DOCKER_FILE ?= Dockerfile.dev
docker-nerpanet-dev: VISOR_NETWORK_TARGET ?= nerpanet
docker-nerpanet-dev: VISOR_IMAGE_TAG ?= $(COMMIT)-nerpanet-dev
docker-nerpanet-dev: docker-build-image-template

.PHONY: docker-nerpanet-dev-push
docker-nerpanet-dev-push: docker-nerpanet-dev docker-tag-and-push-template

# 2K
.PHONY: docker-2k
docker-2k: VISOR_DOCKER_FILE ?= Dockerfile
docker-2k: VISOR_NETWORK_TARGET ?= 2k
docker-2k: VISOR_IMAGE_TAG ?= $(COMMIT)-2k
docker-2k: docker-build-image-template

.PHONY: docker-2k-push
docker-2k-push: docker-2k docker-tag-and-push-template

.PHONY: docker-2k-dev
docker-2k-dev: VISOR_DOCKER_FILE ?= Dockerfile.dev
docker-2k-dev: VISOR_NETWORK_TARGET ?= 2k
docker-2k-dev: VISOR_IMAGE_TAG ?= $(COMMIT)-2k-dev
docker-2k-dev: docker-build-image-template

.PHONY: docker-2k-dev-push
docker-2k-dev-push: docker-2k-dev docker-tag-and-push-template


.PHONY: docker-build-image-template
docker-build-image-template:
	@echo "Building visor docker image for '$(VISOR_NETWORK_TARGET)'..."
	docker build -f $(VISOR_DOCKER_FILE) \
		--build-arg VISOR_NETWORK_TARGET=$(VISOR_NETWORK_TARGET) \
		--build-arg GO_BUILD_IMAGE=$(GO_BUILD_IMAGE) \
		-t $(VISOR_IMAGE_NAME) \
		-t $(VISOR_IMAGE_NAME):latest \
		-t $(VISOR_IMAGE_NAME):$(VISOR_IMAGE_TAG) \
		.

.PHONY: docker-tag-and-push-template
docker-tag-and-push-template:
	./scripts/push-docker-tags.sh $(VISOR_IMAGE_NAME) deprecatedvalue $(VISOR_IMAGE_TAG)

.PHONY: docker-image
docker-image: docker-mainnet
	@echo "*** Deprecated make target 'docker-image': Please use 'make docker-mainnet' instead. ***"
