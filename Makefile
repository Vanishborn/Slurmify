BINARY_NAME=slurmify
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GOPATH=$(shell go env GOPATH)
DIST_DIR=dist
PLATFORMS = \
	linux_amd64 \
	linux_arm64 \
	linux_ppc64le \
	darwin_arm64 \
	darwin_amd64

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

release: clean
	@echo "Building release artifacts for version: $(VERSION)"
	@mkdir -p $(DIST_DIR)

	@set -e; for platform in $(PLATFORMS); do \
		OS=$${platform%_*}; \
		ARCH=$${platform#*_}; \
		echo "Building $$OS $$ARCH..."; \
		mkdir -p $(DIST_DIR)/$$platform; \
		GOOS=$$OS GOARCH=$$ARCH go build -ldflags "-X main.version=$(VERSION)" -o $(DIST_DIR)/$$platform/$(BINARY_NAME); \
		cp LICENSE $(DIST_DIR)/$$platform/; \
		tar -czf $(DIST_DIR)/Slurmify_$(VERSION)_$${platform}.tar.gz -C $(DIST_DIR)/$$platform $(BINARY_NAME) LICENSE; \
	done

	@echo "Generating Checksums..."
	@cd $(DIST_DIR) && (shasum -a 256 *.tar.gz 2>/dev/null || sha256sum *.tar.gz) > checksums.txt

	@echo "Cleaning temp folders..."
	@rm -rf $(DIST_DIR)/linux_* $(DIST_DIR)/darwin_*

	@echo "Release artifacts ready in $(DIST_DIR)/"
	@ls -lh $(DIST_DIR)

help:
	@echo "Choose a command:"
	@echo "  make build    - Build binary in current directory"
	@echo "  make install  - Install binary to system"
	@echo "  make clean    - Remove local binary"
	@echo "  make all      - Clean, build, and install"
	@echo "  make release  - Build release artifacts and checksums"
	@echo "  make help     - Show this help message"
