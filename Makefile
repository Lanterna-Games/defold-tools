BINARIES := defold-vet defoldpls
PACKAGE := github.com/Lanterna-Games/defold-tools
VERSION ?=
PREFIX ?= $(HOME)/.local
LDFLAGS := -s -w -X $(PACKAGE)/internal/cli.versionSetByLinker=$(VERSION) -X $(PACKAGE)/internal/pls.versionSetByLinker=$(VERSION)

.DEFAULT_GOAL := help
.PHONY: help build vsix icon format format-check lint lint-config vet test race cover tidy-check scripts-check commits-check vulnerabilities check install uninstall clean

help: ## List the targets
	@awk 'BEGIN {FS = ":.*## "} /^[a-z-]+:.*## / {printf "  %-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the programs into build/
	go build -trimpath -ldflags "$(LDFLAGS)" -o build/ ./cmd/...

format: ## Format the Go sources
	golangci-lint fmt

format-check: ## Fail if a Go source is not formatted
	golangci-lint fmt --diff

lint-config: ## Fail if .golangci.yml is not valid
	golangci-lint config verify

lint: lint-config ## Run the linters
	golangci-lint run

vet: ## Run go vet
	go vet ./...

test: ## Run the tests
	go test ./...

race: ## Run the tests with the race detector
	go test -race ./...

cover: ## Write coverage.out and print the total
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

tidy-check: ## Fail if go.mod or go.sum would change
	go mod tidy -diff

scripts-check: ## Lint the shell scripts
	shellcheck scripts/*.sh

vulnerabilities: ## Check dependencies against the Go vulnerability database
	go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...

commits-check: ## Check the subjects of the commits that are not on main yet
	scripts/check-commit-messages.sh --range origin/main..HEAD

check: format-check lint vet race tidy-check scripts-check commits-check build ## Every check, in one command

vsix: ## Package the VS Code extension into build/
	go run ./editors/vscode/pack -output build/defold-tools.vsix

icon: ## Draw the icon of the VS Code extension
	go run ./editors/vscode/icon -output editors/vscode/icon.png

install: build ## Install into $(PREFIX)/bin
	for binary in $(BINARIES); do install -Dm755 build/$$binary $(PREFIX)/bin/$$binary; done

uninstall: ## Remove the installed programs
	for binary in $(BINARIES); do rm -f $(PREFIX)/bin/$$binary; done

clean: ## Remove build output
	rm -rf build dist coverage.out coverage.html
