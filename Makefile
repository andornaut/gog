# https://www.gnu.org/prep/standards/html_node/Directory-Variables.html#Directory-Variables
PREFIX    ?= /usr/local
BINPREFIX ?= $(PREFIX)/bin
DISTDIR   := dist
TARGET    := gog
# The flags .goreleaser.yaml builds a tag with, so a binary built here is the
# binary a release ships rather than an unstripped one carrying local paths.
LDFLAGS   := -s -w

.DEFAULT_GOAL := check
CHECK_TASKS := lint test build
TEST_OUTPUTS := coverage.txt
BUILD_OUTPUTS := gog
CONTAINER_BUILD_ARGS := GO_VERSION=$(shell awk '$$1 == "go" {print $$2; exit}' go.mod)
CHECK_NETWORK_LINT := bridge

override COMMAND := $(value COMMAND)
override TEST_ARGS := $(value TEST_ARGS)

export CHECK_TASKS BUILD_OUTPUTS TEST_OUTPUTS CONTAINER_BUILD_ARGS CHECK_NETWORK_LINT TEST_ARGS COMMAND

export CHECK_TIMEOUT_SECONDS CHECK_MEMORY CHECK_SCRATCH_SIZE CHECK_WORK_SIZE CHECK_TMP_SIZE CHECK_NETWORK_LINT CONTAINER_IMAGE
export CHECK_ADMISSION

.PHONY: all check check-info lint test build exec coverage fmt install clean uninstall

all: build

check lint test build exec:
	@./tests/container/run $@

check-info:
	@./tests/container/run info

coverage: test

fmt:
	@./tests/container/run exec golangci-lint fmt

$(TARGET): build

clean:
	go clean
	rm -rf $(DISTDIR)
	rm -f coverage.txt

install: build
	sudo mkdir -p "$(DESTDIR)$(BINPREFIX)"
	sudo cp -pf $(TARGET) "$(DESTDIR)$(BINPREFIX)/"

uninstall:
	sudo rm -f "$(DESTDIR)$(BINPREFIX)/$(TARGET)"

.PHONY: test-native
test-native:
	@./tests/native/run
