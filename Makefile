.PHONY: build run test lint clean docker-build docker-run

# App Name
APP_NAME = web-analyzer-api
VERSION ?= $(shell git describe --tags --always --dirty)

# Go parameters
GOCMD = go
GOBUILD = $(GOCMD) build
GORUN = $(GOCMD) run
GOTEST = $(GOCMD) test
GOCLEAN = $(GOCMD) clean
GOGET = $(GOCMD) get
GOMOD = $(GOCMD) mod
GOLINT = golangci-lint

# Main package path
MAIN_PACKAGE = ./cmd/api

# Build flags
LDFLAGS = -ldflags "-X main.version=$(VERSION)"

# Docker parameters
DOCKER_IMAGE = web-analyzer-api
DOCKER_TAG = latest

all: test build

build:
	$(GOBUILD) $(LDFLAGS) -o $(APP_NAME) $(MAIN_PACKAGE)

run:
	$(GORUN) $(MAIN_PACKAGE)

test:
	$(GOTEST) -v ./...

test-coverage:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

lint:
	$(GOLINT) run

clean:
	$(GOCLEAN)
	rm -f $(APP_NAME)
	rm -f coverage.out coverage.html

docker-build:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

docker-run:
	docker run -p 8080:8080 $(DOCKER_IMAGE):$(DOCKER_TAG)

tidy:
	$(GOMOD) tidy

deps:
	$(GOMOD) download

help:
	@echo "make              - Run tests and build"
	@echo "make build        - Build the application"
	@echo "make run          - Run the application"
	@echo "make test         - Run tests"
	@echo "make test-coverage - Run tests with coverage report"
	@echo "make lint         - Run linters"
	@echo "make clean        - Clean build files"
	@echo "make docker-build - Build Docker image"
	@echo "make docker-run   - Run Docker container"
	@echo "make tidy         - Tidy and verify dependencies"
	@echo "make deps         - Download dependencies"