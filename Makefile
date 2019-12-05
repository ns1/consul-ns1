PROJECT_NAME := consul-ns1
PKG := github.com/nsone/$(PROJECT_NAME)
GIT_COMMIT = $(shell git rev-parse --short HEAD)
GIT_DIRTY = $(shell test -n "$(git status --porcelain)" && echo "+CHANGES")
GIT_DESCRIBE = $(shell git describe --tags --always)
GIT_IMPORT = $(PKG)/version
GOLDFLAGS = "-X ${GIT_IMPORT}.GitCommit=${GIT_COMMIT}${GIT_DIRTY} -X ${GIT_IMPORT}.GitDescribe=${GIT_DESCRIBE}"

.PHONY: all
all: build

.PHONY: dep
dep: ## Get dependencies
	@go mod download

.PHONY: lint
lint: ## Lint Golang files
	@golint -set_exit_status ./...

.PHONY: test
test: ## Run unittests
	@go test -v ./...

.PHONY: build
build: dep ## Build the binary
	@go build -o consul-ns1 -ldflags $(GOLDFLAGS) .

.PHONY: goldflags
goldflags: ## echo GO LD Flags for setting build env var
	@echo $(GOLDFLAGS)
