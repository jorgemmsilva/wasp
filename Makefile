GIT_REF_TAG := $(shell git describe --tags)
BUILD_TAGS = rocksdb
ifdef OS
# windows
BUILD_LD_FLAGS = "-X=github.com/iotaledger/wasp/components/app.Version=$(GIT_REF_TAG)"
else
ifeq ($(shell uname -m), arm64)
BUILD_LD_FLAGS = "-X=github.com/iotaledger/wasp/components/app.Version=$(GIT_REF_TAG) -extldflags \"-Wa,--noexecstack\""
else
BUILD_LD_FLAGS = "-X=github.com/iotaledger/wasp/components/app.Version=$(GIT_REF_TAG) -extldflags \"-z noexecstack\""
endif
endif
DOCKER_BUILD_ARGS = # E.g. make docker-build "DOCKER_BUILD_ARGS=--tag wasp:devel"

#
# You can override these e.g. as
#     make test TEST_PKG=./packages/vm/core/testcore/ TEST_ARG="-v --run TestAccessNodes"
#
TEST_PKG=./...
TEST_ARG=

BUILD_PKGS ?= ./ ./tools/cluster/wasp-cluster/
BUILD_CMD=go build -o . -tags $(BUILD_TAGS) -ldflags $(BUILD_LD_FLAGS)
INSTALL_CMD=go install -tags $(BUILD_TAGS) -ldflags $(BUILD_LD_FLAGS)
WASP_CLI_TAGS = no_wasmhost

# Docker image name and tag
DOCKER_IMAGE_NAME=wasp
DOCKER_IMAGE_TAG=develop

all: build-lint

wasm:
	bash contracts/wasm/scripts/schema_all.sh

compile-solidity:
	cd packages/vm/core/evm/iscmagic && go generate
	cd packages/evm/evmtest && go generate

build-cli:
	cd tools/wasp-cli && go mod tidy && go build -ldflags $(BUILD_LD_FLAGS) -tags ${WASP_CLI_TAGS} -o ../../

build-full: build-cli
	$(BUILD_CMD) ./...

build: build-cli
	$(BUILD_CMD) $(BUILD_PKGS)

build-lint: build lint

gendoc:
	./scripts/gendoc.sh

test-full: install
	go test -tags $(BUILD_TAGS),runheavy -ldflags $(BUILD_LD_FLAGS) ./... --timeout 60m --count 1 -failfast

test: install
	go test -tags $(BUILD_TAGS) -ldflags $(BUILD_LD_FLAGS) $(TEST_PKG) --timeout 90m --count 1 -failfast  $(TEST_ARG)

# TODO: once all test packages are passing, uncomment this:
# SHORT_TESTS = $(shell go list ./... | grep -v github.com/iotaledger/wasp/contracts/wasm | sed 's|github.com/iotaledger/wasp|.|')

# TODO: once all test packages are passing, remove this
SHORT_TESTS = \
./components/database \
./components/webapi \
./contracts/native/inccounter \
./packages/authentication \
./packages/cryptolib \
./packages/database \
./packages/evm/jsonrpc/jsonrpctest \
./packages/hashing \
./packages/isc \
./packages/isc/rotate \
./packages/kv/buffered \
./packages/kv/codec \
./packages/kv/collections \
./packages/kv/dict \
./packages/metrics \
./packages/onchangemap \
./packages/origin \
./packages/registry \
./packages/shutdown \
./packages/solo \
./packages/solo/examples \
./packages/solo/solotest \
./packages/state \
./packages/tcrypto/bls \
./packages/testutil \
./packages/testutil/testlogger \
./packages/testutil/testpeers \
./packages/testutil/utxodb \
./packages/transaction \
./packages/trie \
./packages/trie/test \
./packages/util \
./packages/util/byz_quorum \
./packages/util/pipe \
./packages/vm/core/accounts \
./packages/vm/core/blob \
./packages/vm/core/blocklog \
./packages/vm/core/evm/emulator \
./packages/vm/core/evm/evmtest \
./packages/vm/core/testcore \
./packages/vm/gas \
./packages/vm/vmimpl \
./packages/vm/vmtxbuilder \
./packages/webapi/controllers/node \
./packages/webapi/test \
./packages/webapi/websocket \
./packages/webapi/websocket/commands \
./tools/evm/iscutils \
./tools/schema/model/yaml \



# TODO: move these to SHORT_TESTS when they are passing
SHORT_TESTS_WIP = \
./documentation/tutorial-examples/test \
./packages/chain \
./packages/chain/chainmanager \
./packages/chain/cmt_log \
./packages/chain/cons \
./packages/chain/cons/bp \
./packages/chain/cons/cons_gr \
./packages/chain/dss \
./packages/chain/mempool \
./packages/chain/mempool/distsync \
./packages/chain/statemanager \
./packages/chain/statemanager/sm_gpa \
./packages/chain/statemanager/sm_gpa/sm_gpa_utils \
./packages/chain/statemanager/sm_gpa/sm_messages \
./packages/chain/statemanager/sm_snapshots \
./packages/chain/statemanager/sm_utils \
./packages/chains \
./packages/chains/access_mgr \
./packages/chains/access_mgr/am_dist \
./packages/dkg \
./packages/gpa \
./packages/gpa/aba/mostefaoui \
./packages/gpa/acs \
./packages/gpa/acss \
./packages/gpa/acss/crypto \
./packages/gpa/adkg \
./packages/gpa/adkg/nonce \
./packages/gpa/cc/blssig \
./packages/gpa/cc/semi \
./packages/gpa/rbc/bracha \
./packages/peering \
./packages/peering/domain \
./packages/peering/group \
./packages/peering/lpp \
./packages/util/l1starter \
./packages/vm/core/testcore/sbtests \
./packages/wasmvm/wasmclient/go/test \
./tools/cluster \
./tools/cluster/tests \


test-short:
	@for p in $(SHORT_TESTS); do \
		if [[ -n `find $$p -name '*_test.go' -print -quit` ]]; then \
			go test -tags $(BUILD_TAGS) -ldflags $(BUILD_LD_FLAGS) --short --count 1 -failfast $$p || exit 1; \
		else \
		    echo "$$p: no test files"; \
		fi \
	done

install-cli:
	cd tools/wasp-cli && go mod tidy && go install -ldflags $(BUILD_LD_FLAGS)

install-full: install-cli
	$(INSTALL_CMD) ./...

install: install-cli install-pkgs

install-pkgs:
	$(INSTALL_CMD) $(BUILD_PKGS)

lint: lint-wasp-cli
	golangci-lint run --timeout 5m

lint-wasp-cli:
	cd ./tools/wasp-cli && golangci-lint run --timeout 5m

apiclient:
	./clients/apiclient/generate_client.sh

apiclient-docker:
	./clients/apiclient/generate_client.sh docker

gofumpt-list:
	gofumpt -l ./

docker-build:
	DOCKER_BUILDKIT=1 docker build ${DOCKER_BUILD_ARGS} \
		--build-arg BUILD_TAGS=${BUILD_TAGS} \
		--build-arg BUILD_LD_FLAGS=${BUILD_LD_FLAGS} \
		--tag iotaledger/$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG) \
		.

docker-check-push-deps:
    ifndef DOCKER_USERNAME
	    $(error DOCKER_USERNAME is undefined)
    endif
    ifndef DOCKER_ACCESS_TOKEN
	    $(error DOCKER_ACCESS_TOKEN is undefined)
    endif

docker-push:
	echo "$(DOCKER_ACCESS_TOKEN)" | docker login --username $(DOCKER_USERNAME) --password-stdin
	docker tag iotaledger/$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG) $(DOCKER_USERNAME)/$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)
	docker push $(DOCKER_USERNAME)/$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)

docker-build-push: docker-check-push-deps docker-build docker-push

.PHONY: all wasm compile-solidity build-cli build-full build build-lint test-full test test-short install-cli install-full install lint gofumpt-list docker-build deps-versions
