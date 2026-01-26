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
	# @$(MAKE) plugin

#? plugin: Build the suspicious txfilter plugin.
# The plugin must be built with the exact same Go version, build tags, and go.mod as the test binary.
plugin:
	@echo "Building suspicious txfilter plugin..."
	@go build -buildmode=plugin -trimpath -tags urfave_cli_no_docs,ckzg -o ./build/bin/suspicious_txfilter.so txfilter/dummy_plugin.go
	@echo "Plugin built successfully."

#? plugin-test: Build / sign / create metadata for the suspicious txfilter plugin for testing.
plugin-test:
	@echo "Building suspicious txfilter plugin for testing..."
	@go build -buildmode=plugin -ldflags "-X main.version=1.0.0 -X main.blockedByPlugin=false" -o ./txfilter/testdata/suspicious_txfilter-v1.so ./txfilter/dummy_plugin.go
	@go build -buildmode=plugin -ldflags "-X main.version=2.0.0 -X main.blockedByPlugin=true" -o ./txfilter/testdata/suspicious_txfilter-v2.so ./txfilter/dummy_plugin.go
	@echo "Plugin for testing built successfully."
	@echo "Sign the plugin...(the key password is empty)"
	@cosign sign-blob --key ./txfilter/testdata/cosign-test.key --bundle ./txfilter/testdata/suspicious_txfilter-v1.so.bundle ./txfilter/testdata/suspicious_txfilter-v1.so
	@cosign sign-blob --key ./txfilter/testdata/cosign-test.key --bundle ./txfilter/testdata/suspicious_txfilter-v2.so.bundle ./txfilter/testdata/suspicious_txfilter-v2.so
	@echo "Plugin signed successfully."
	@echo "Create plugin metadata..."
	@go run txfilter/plugin_metadata_creator.go ./txfilter/testdata/suspicious_txfilter-v1.so.bundle ./txfilter/testdata/cosign-test.pub ./txfilter/testdata/suspicious_txfilter-v1.json 1.0.0
	@go run txfilter/plugin_metadata_creator.go ./txfilter/testdata/suspicious_txfilter-v2.so.bundle ./txfilter/testdata/cosign-test.pub ./txfilter/testdata/suspicious_txfilter-v2.json 2.0.0
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
