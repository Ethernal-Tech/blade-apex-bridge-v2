.PHONY: download-submodules
download-submodules: check-git
	git submodule init
	git submodule update

.PHONY: check-git
check-git:
	@which git > /dev/null || (echo "git is not installed. Please install and try again."; exit 1)

.PHONY: check-go
check-go:
	@which go > /dev/null || (echo "Go is not installed.. Please install and try again."; exit 1)

.PHONY: check-protoc
check-protoc:
	@which protoc > /dev/null || (echo "protoc is not installed. Please install and try again."; exit 1)

.PHONY: check-lint
check-lint:
	@which golangci-lint > /dev/null || (echo "golangci-lint is not installed. Please install and try again."; exit 1)

.PHONY: check-npm
check-npm:
	@which npm > /dev/null || (echo "npm is not installed. Please install and try again."; exit 1)

.PHONY: bindata
bindata: check-go
	go-bindata -pkg chain -o ./chain/chain_bindata.go ./chain/chains

.PHONY: protoc
protoc: check-protoc
	protoc --go_out=. --go-grpc_out=. -I . -I=./validate --validate_out="lang=go:." \
	 ./server/proto/*.proto \
	 ./network/proto/*.proto \
	 ./txpool/proto/*.proto	\
	 ./consensus/polybft/**/*.proto

.PHONY: build
build: check-go check-git
	$(eval COMMIT_HASH = $(shell git rev-parse HEAD))
	$(eval VERSION = $(shell git describe --tags --abbrev=0 ${COMMIT_HASH}))
	$(eval BRANCH = $(shell git rev-parse --abbrev-ref HEAD | tr -d '\040\011\012\015\n'))
	$(eval TIME = $(shell date))
	go build -o blade -ldflags="\
		-X 'github.com/0xPolygon/polygon-edge/versioning.Version=$(VERSION)' \
		-X 'github.com/0xPolygon/polygon-edge/versioning.Commit=$(COMMIT_HASH)'\
		-X 'github.com/0xPolygon/polygon-edge/versioning.Branch=$(BRANCH)'\
		-X 'github.com/0xPolygon/polygon-edge/versioning.BuildTime=$(TIME)'" \
	main.go

.PHONY: lint
lint: check-lint
	golangci-lint run --config .golangci.yml --whole-files

.PHONY: generate-bsd-licenses
generate-bsd-licenses: check-git
	./generate_dependency_licenses.sh BSD-3-Clause,BSD-2-Clause > ./licenses/bsd_licenses.json

.PHONY: unit-test
unit-test: check-go
	go test -race -shuffle=on -coverprofile coverage.out -timeout 20m `go list ./... | grep -v e2e`

.PHONY: benchmark-test
benchmark-test: check-go
	go test -bench=. -run=^$ `go list ./... | grep -v /e2e`

.PHONY: fuzz-test
fuzz-test: check-go
	./scripts/fuzzAll

.PHONY: test-e2e-legacy
test-e2e-legacy: check-go
	go build -race -o artifacts/blade .
	env EDGE_BINARY=${PWD}/artifacts/blade go test -v -timeout=30m ./e2e/...

.PHONY: test-e2e-polybft
test-e2e-polybft: check-go
	@TESTS=`go test -list . ./e2e-polybft/e2e/... | grep '^Test' | grep -v ApexBridge | paste -sd '|' - | tr -d '\n'`; \
	go build -o artifacts/blade .; \
	env EDGE_BINARY=${PWD}/artifacts/blade E2E_TESTS=true E2E_LOGS=true \
	go test -v -timeout=5h ./e2e-polybft/e2e/... -run "$${TESTS}"

.PHONY: test-e2e-apex-bridge
test-e2e-apex-bridge: check-go
	go build -o artifacts/blade .
	env EDGE_BINARY=${PWD}/artifacts/blade E2E_TESTS=true E2E_LOGS=true \
	go test -v -timeout=7h ./e2e-polybft/e2e/... -run "ApexBridge"

.PHONY: test-property-polybft
test-property-polybft: check-go
	go build -o artifacts/blade .
	env EDGE_BINARY=${PWD}/artifacts/blade E2E_TESTS=true E2E_LOGS=true go test -v -timeout=30m ./e2e-polybft/property/... \
	-rapid.checks=10

.PHONY: compile-blade-contracts
compile-blade-contracts: check-npm
	cd blade-contracts && npm install && npm run compile
	$(MAKE) generate-smart-contract-bindings

.PHONY: generate-smart-contract-bindings
generate-smart-contract-bindings: check-go
	go run ./consensus/polybft/contractsapi/artifacts-gen/main.go
	go run ./consensus/polybft/contractsapi/bindings-gen/main.go

.PHONY: run-docker
run-docker:
	./scripts/cluster polybft --docker

.PHONY: stop-docker
stop-docker:
	./scripts/cluster polybft --docker stop

.PHONY: destroy-docker
destroy-docker:
	./scripts/cluster polybft --docker destroy

.PHONY: update-apex-contracts
update-apex-contracts:
	git submodule update --remote --init apex-bridge-smartcontracts && \
	cd apex-bridge-smartcontracts/ && npm i && npx hardhat compile && cd .. && \
	go run consensus/polybft/contractsapi/apex-artifacts-gen/main.go && \
	go run consensus/polybft/contractsapi/bindings-gen/main.go

.PHONY: help
help:
	@echo "Available targets:"
	@printf "  %-35s - %s\n" "download-submodules" "Initialize and update Git submodules"
	@printf "  %-35s - %s\n" "bindata" "Generate Go binary data for chain"
	@printf "  %-35s - %s\n" "protoc" "Compile Protocol Buffers files"
	@printf "  %-35s - %s\n" "build" "Build the project"
	@printf "  %-35s - %s\n" "lint" "Run linters on the codebase"
	@printf "  %-35s - %s\n" "generate-bsd-licenses" "Generate BSD licenses"
	@printf "  %-35s - %s\n" "unit-test" "Run unit tests"
	@printf "  %-35s - %s\n" "fuzz-test" "Run fuzz tests"
	@printf "  %-35s - %s\n" "test-e2e-legacy" "Run end-to-end Legacy tests"
	@printf "  %-35s - %s\n" "test-e2e-polybft" "Run end-to-end tests for PolyBFT"
	@printf "  %-35s - %s\n" "test-property-polybft" "Run property tests for PolyBFT"
	@printf "  %-35s - %s\n" "compile-blade-contracts" "Compile blade contracts"
	@printf "  %-35s - %s\n" "generate-smart-contract-bindings" "Generate smart contract bindings"
	@printf "  %-35s - %s\n" "test-e2e-apex-bridge" "Run end-to-end tests for Apex Bridge"
	@printf "  %-35s - %s\n" "update-apex-contracts" "Update Apex Bridge smart contracts and bindings"
	@printf "  %-35s - %s\n" "run-docker" "Run Docker cluster for PolyBFT"
	@printf "  %-35s - %s\n" "stop-docker" "Stop Docker cluster for PolyBFT"
	@printf "  %-35s - %s\n" "destroy-docker" "Destroy Docker cluster for PolyBFT"
