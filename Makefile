# Star Terminal Makefile
# This Makefile provides a convenient interface to the Task-based build system
# and adds additional development utilities.

.PHONY: help init dev start package clean test lint format check build-backend \
        build-server build-wsh generate storybook docsite version \
        test-go test-frontend coverage install-deps \
        check-deps security-audit clean-all

# Default target
.DEFAULT_GOAL := help

# Get version from package.json
VERSION := $(shell node version.cjs)

# Platform detection
ifeq ($(OS),Windows_NT)
    PLATFORM := windows
    RM := powershell Remove-Item -Force -ErrorAction SilentlyContinue
    RMRF := powershell Remove-Item -Force -Recurse -ErrorAction SilentlyContinue
else
    UNAME_S := $(shell uname -s)
    ifeq ($(UNAME_S),Darwin)
        PLATFORM := darwin
    else
        PLATFORM := linux
    endif
    RM := rm -f
    RMRF := rm -rf
endif

## help: Display this help message
help:
	@echo "Star Terminal Build System"
	@echo "=========================="
	@echo ""
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ""
	@echo "Current platform: $(PLATFORM)"
	@echo "Current version: $(VERSION)"

## init: Initialize project dependencies
init:
	@echo "🚀 Initializing Star Terminal development environment..."
	task init

## install-deps: Install all dependencies (alias for init)
install-deps: init

## dev: Start development server with hot reload
dev:
	@echo "🔥 Starting development server..."
	task dev

## start: Start application without development server
start:
	@echo "▶️  Starting Star Terminal..."
	task start

## build-backend: Build Go backend components
build-backend:
	@echo "🔨 Building backend components..."
	task build:backend

## build-server: Build starsrv component only
build-server:
	@echo "🔨 Building starsrv..."
	task build:server

## build-wsh: Build wsh component only
build-wsh:
	@echo "🔨 Building wsh..."
	task build:wsh

## generate: Generate TypeScript bindings from Go
generate:
	@echo "🔄 Generating TypeScript bindings..."
	task generate

## package: Package application for current platform
package:
	@echo "📦 Packaging Star Terminal for $(PLATFORM)..."
	task package

## test: Run all tests
test: test-go test-frontend

## test-go: Run Go tests
test-go:
	@echo "🧪 Running Go tests..."
	go test ./pkg/... -v

## test-frontend: Run frontend tests
test-frontend:
	@echo "🧪 Running frontend tests..."
	yarn test

## coverage: Generate test coverage report
coverage:
	@echo "📊 Generating coverage report..."
	yarn coverage

## lint: Lint all code
lint:
	@echo "🔍 Linting code..."
	yarn eslint . --ext .ts,.tsx,.js,.jsx
	go vet ./...

## format: Format all code
format:
	@echo "✨ Formatting code..."
	yarn prettier --write .
	go fmt ./...

## check: Run linting and formatting checks
check:
	@echo "✅ Running code quality checks..."
	yarn prettier --check .
	yarn eslint . --ext .ts,.tsx,.js,.jsx
	go vet ./...
	go mod verify

## security-audit: Run security audit
security-audit:
	@echo "🔒 Running security audit..."
	yarn audit
	go list -json -m all | nancy sleuth

## storybook: Start Storybook development server
storybook:
	@echo "📚 Starting Storybook..."
	task storybook

## storybook-build: Build Storybook static files
storybook-build:
	@echo "📚 Building Storybook..."
	task storybook:build

## docsite: Start documentation site
docsite:
	@echo "📖 Starting documentation site..."
	task docsite

## docsite-build: Build documentation site
docsite-build:
	@echo "📖 Building documentation site..."
	task docsite:build:public

## version: Show current version
version:
	@echo "Current version: $(VERSION)"

## bump-patch: Bump patch version
bump-patch:
	@echo "⬆️  Bumping patch version..."
	task version -- patch

## bump-minor: Bump minor version
bump-minor:
	@echo "⬆️  Bumping minor version..."
	task version -- minor

## bump-major: Bump major version
bump-major:
	@echo "⬆️  Bumping major version..."
	task version -- major

## clean: Clean build artifacts
clean:
	@echo "🧹 Cleaning build artifacts..."
	$(RMRF) dist
	$(RMRF) make
	$(RMRF) node_modules/.cache
	$(RMRF) .yarn/cache

## clean-all: Clean all generated files including dependencies
clean-all: clean
	@echo "🧹 Deep cleaning..."
	$(RMRF) node_modules
	$(RMRF) .yarn/install-state.gz
	$(RMRF) docs/node_modules
	$(RMRF) docs/build

## check-deps: Check for dependency updates
check-deps:
	@echo "🔍 Checking for dependency updates..."
	yarn outdated || true
	go list -u -m all || true

## install-wsh: Quick install wsh for development (macOS ARM64)
install-wsh:
	@echo "🔧 Installing wsh for development..."
	task dev:installwsh

## clear-config: Clear development configuration
clear-config:
	@echo "🗑️  Clearing development configuration..."
	task dev:clearconfig

## clear-data: Clear development data
clear-data:
	@echo "🗑️  Clearing development data..."
	task dev:cleardata

## schema: Build configuration schema
schema:
	@echo "📋 Building configuration schema..."
	task build:schema

## validate: Validate project setup
validate:
	@echo "✅ Validating project setup..."
	@command -v node >/dev/null 2>&1 || { echo "❌ Node.js is not installed"; exit 1; }
	@command -v go >/dev/null 2>&1 || { echo "❌ Go is not installed"; exit 1; }
	@command -v task >/dev/null 2>&1 || { echo "❌ Task is not installed"; exit 1; }
	@command -v yarn >/dev/null 2>&1 || { echo "❌ Yarn is not available"; exit 1; }
	@echo "✅ All required tools are available"

## doctor: Run project health check
doctor: validate check-deps
	@echo "🩺 Running project health check..."
	@echo "Node version: $(shell node --version)"
	@echo "Go version: $(shell go version)"
	@echo "Task version: $(shell task --version)"
	@echo "Yarn version: $(shell yarn --version)"
	@echo ""
	@echo "✅ Project health check complete"

# Development workflow shortcuts
## dev-reset: Reset development environment
dev-reset: clear-config clear-data clean
	@echo "🔄 Resetting development environment..."
	$(MAKE) init

## quick-build: Quick build for development
quick-build: generate build-backend

## full-build: Complete build pipeline
full-build: clean init generate build-backend package

## ci: Continuous integration workflow
ci: validate lint test coverage

## release-prep: Prepare for release
release-prep: clean-all init lint test coverage package

# Platform-specific targets
ifeq ($(PLATFORM),darwin)
## install-macos-deps: Install macOS-specific dependencies
install-macos-deps:
	@echo "🍎 Installing macOS dependencies..."
	@command -v brew >/dev/null 2>&1 || { echo "❌ Homebrew is not installed"; exit 1; }
	brew install task go node
endif

ifeq ($(PLATFORM),linux)
## install-linux-deps: Install Linux-specific dependencies
install-linux-deps:
	@echo "🐧 Installing Linux dependencies..."
	@echo "Please install: zip, zig, task, go, node"
	@echo "See BUILD.md for detailed instructions"
endif

# Help with Task commands
## task-help: Show available Task commands
task-help:
	@echo "📋 Available Task commands:"
	task --list

# Make sure we don't conflict with any files
%::
	@:
