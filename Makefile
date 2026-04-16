# This Makefile is meant to be used by people that do not usually work
# with Go source code. If you know what GOPATH is then you probably
# don't need to bother with make.

.PHONY: geth evm faucet all test truffle-test lint fmt clean devtools help
.PHONY: docker

GOBIN = ./build/bin
GO ?= latest
GORUN = go run
GIT_COMMIT=$(shell git rev-parse HEAD)
GIT_COMMIT_DATE=$(shell git log -n1 --pretty='format:%cd' --date=format:'%Y%m%d')

#? geth: Build geth.
geth:
	$(GORUN) build/ci.go install ./cmd/geth
	@echo "Done building."
	@echo "Run \"$(GOBIN)/geth\" to launch geth."

#? plugin: Build the suspicious txfilter plugin.
# Uses build/ci.go to share toolchain setup and build tags with the main binary.
plugin:
	@echo "Building suspicious txfilter plugin..."
	@$(GORUN) build/ci.go plugin
	@echo "Plugin built successfully."

#? plugin-test: Build / sign / create metadata for the suspicious txfilter plugin for testing.
#?              Usage: make plugin-test [BLOCKED=true]
BLOCKED ?= false
PLUGIN_TEST_DIR = ./txfilter/testdata
PLUGIN_TEST_SO = $(PLUGIN_TEST_DIR)/suspicious_txfilter.so
PLUGIN_TEST_BUNDLE = $(PLUGIN_TEST_SO).bundle
PLUGIN_TEST_JSON = $(PLUGIN_TEST_DIR)/suspicious_txfilter.json
plugin-test:
	@echo "Building suspicious txfilter plugin (blocked=$(BLOCKED))..."
	@$(GORUN) build/ci.go plugin -version 1.0.0 $(if $(filter true,$(BLOCKED)),-blocked) -o $(PLUGIN_TEST_SO)
	@echo "Plugin built successfully."

	@echo "Signing plugin...(the key password is empty)"
	@COSIGN_PASSWORD="" cosign sign-blob --yes --new-bundle-format --tlog-upload=false --key $(PLUGIN_TEST_DIR)/cosign-test.key --bundle $(PLUGIN_TEST_BUNDLE) $(PLUGIN_TEST_SO)
	@echo "Plugin signed successfully."

	@echo "Creating plugin metadata..."
	@go run txfilter/cmd/createmeta/metadata_creator.go $(PLUGIN_TEST_BUNDLE) $(PLUGIN_TEST_DIR)/cosign-test.pub $(PLUGIN_TEST_JSON) 1.0.0
	@echo "Plugin metadata created successfully."

#? faucet: Build faucet
faucet:
	$(GORUN) build/ci.go install ./cmd/faucet
	@echo "Done building faucet"

#? evm: Build evm.
evm:
	$(GORUN) build/ci.go install ./cmd/evm
	@echo "Done building."
	@echo "Run \"$(GOBIN)/evm\" to launch evm."

#? all: Build all packages and executables.
all:
	$(GORUN) build/ci.go install

#? test: Run the tests.
test: all
	$(GORUN) build/ci.go test -timeout 1h

#? test-oasys: Run Oasys consensus tests
test-oasys:
	go test -v ./consensus/oasys/...

#? truffle-test: Run the integration test.
truffle-test:
	rm -rf ./tests/truffle/storage/bsc-validator1
	rm -rf ./tests/truffle/storage/bsc-rpc
	docker build . -f ./docker/Dockerfile --target bsc -t bsc
	docker build . -f ./docker/Dockerfile --target bsc-genesis -t bsc-genesis
	docker build . -f ./docker/Dockerfile.truffle -t truffle-test
	docker compose -f ./tests/truffle/docker-compose.yml up genesis
	docker compose -f ./tests/truffle/docker-compose.yml up -d bsc-rpc bsc-validator1
	sleep 60
	docker compose -f ./tests/truffle/docker-compose.yml up --exit-code-from truffle-test truffle-test
	docker compose -f ./tests/truffle/docker-compose.yml down

#? lint: Run certain pre-selected linters.
lint: ## Run linters.
	$(GORUN) build/ci.go lint

#? fmt: Ensure consistent code formatting.
fmt:
	gofmt -s -w $(shell find . -name "*.go")

#? clean: Clean go cache, built executables, and the auto generated folder.
clean:
	go clean -cache
	rm -fr build/_workspace/pkg/ $(GOBIN)/*

# The devtools target installs tools required for 'go generate'.
# You need to put $GOBIN (or $GOPATH/bin) in your PATH to use 'go generate'.

#? devtools: Install recommended developer tools.
devtools:
	env GOBIN= go install golang.org/x/tools/cmd/stringer@latest
	env GOBIN= go install github.com/fjl/gencodec@latest
	env GOBIN= go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	env GOBIN= go install ./cmd/abigen
	@type "solc" 2> /dev/null || echo 'Please install solc'
	@type "protoc" 2> /dev/null || echo 'Please install protoc'

#? help: Build docker image
docker:
	docker build --pull -t bnb-chain/bsc:latest -f Dockerfile .

#? help: Get more info on make commands.
help: Makefile
	@echo ''
	@echo 'Usage:'
	@echo '  make [target]'
	@echo ''
	@echo 'Targets:'
	@sed -n 's/^#?//p' $< | column -t -s ':' |  sort | sed -e 's/^/ /'
