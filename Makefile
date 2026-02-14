BINARY_NAME=slurmify
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

default: build

all: clean build install

build:
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY_NAME)

install:
	@echo "Installing to $(GOPATH)/bin/..."
	go install -ldflags "-X main.version=$(VERSION)"

clean:
	@echo "Cleaning local binary..."
	rm -f $(BINARY_NAME)

help:
	@echo "Choose a command:"
	@echo "  make build    - Build binary in current directory"
	@echo "  make install  - Install binary to system"
	@echo "  make clean    - Remove local binary"
	@echo "  make all      - Clean, build, and install"
	@echo "  make help     - Show this help message"
