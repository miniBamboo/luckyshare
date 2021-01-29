PACKAGE = github.com/miniBamboo/luckyshare

GIT_COMMIT = $(shell git --no-pager log --pretty="%h" -n 1)
GIT_TAG = $(shell git tag -l --points-at HEAD)
LUCKYSHARE_VERSION = $(shell cat cmd/luckyshare/VERSION)
BOOTNODE_VERSION = $(shell cat cmd/bootnode/VERSION)

PACKAGES = `go list ./... | grep -v '/vendor/'`

MAJOR = $(shell go version | cut -d' ' -f3 | cut -b 3- | cut -d. -f1)
MINOR = $(shell go version | cut -d' ' -f3 | cut -b 3- | cut -d. -f2)
export GO111MODULE=on

.PHONY: luckyshare bootnode all clean test

luckyshare:| go_version_check
	@echo "building $@..."
	@go build -v -i -o $(CURDIR)/bin/$@ -ldflags "-X main.version=$(LUCKYSHARE_VERSION) -X main.gitCommit=$(GIT_COMMIT) -X main.gitTag=$(GIT_TAG)" ./cmd/luckyshare
	@echo "done. executable created at 'bin/$@'"

bootnode:| go_version_check
	@echo "building $@..."
	@go build -v -i -o $(CURDIR)/bin/$@ -ldflags "-X main.version=$(DISCO_VERSION) -X main.gitCommit=$(GIT_COMMIT) -X main.gitTag=$(GIT_TAG)" ./cmd/bootnode
	@echo "done. executable created at 'bin/$@'"

dep:| go_version_check
	@go mod download

go_version_check:
	@if test $(MAJOR) -lt 1; then \
		echo "Go 1.13 or higher required"; \
		exit 1; \
	else \
		if test $(MAJOR) -eq 1 -a $(MINOR) -lt 13; then \
			echo "Go 1.13 or higher required"; \
			exit 1; \
		fi \
	fi

all: luckyshare bootnode

clean:
	-rm -rf \
$(CURDIR)/bin/luckyshare \
$(CURDIR)/bin/bootnode 

test:| go_version_check
	@go test -cover $(PACKAGES)

