# https://www.gnu.org/prep/standards/html_node/Directory-Variables.html#Directory-Variables
PREFIX    ?= /usr/local
BINPREFIX ?= $(PREFIX)/bin
DISTDIR   := dist
TARGET    := gog
# The flags .goreleaser.yaml builds a tag with, so a binary built here is the
# binary a release ships rather than an unstripped one carrying local paths.
LDFLAGS   := -s -w

.PHONY: $(TARGET) all build clean coverage fmt install lint test uninstall

all: $(TARGET)

build: $(TARGET)

# CGO_ENABLED=0 on the recipe rather than exported for the file, because the
# test target runs with -race, which needs cgo. Nothing here imports os/user or
# net outside a test, so this only settles how the binary links.
$(TARGET):
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -trimpath -o $@

clean:
	go clean
	rm -rf $(DISTDIR)
	rm -f coverage.txt

install: $(TARGET)
	sudo mkdir -p "$(DESTDIR)$(BINPREFIX)"
	sudo cp -pf $(TARGET) "$(DESTDIR)$(BINPREFIX)/"

test:
	go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...

coverage: test
	go tool cover -func=coverage.txt

fmt:
	golangci-lint fmt

# The checks CI runs, both of them. `run` accepts an unknown key inside
# `linters.settings` and exits 0, which leaves that setting disabled while CI
# stays green, so `config verify` is what rejects a misspelled one.
lint:
	golangci-lint config verify
	golangci-lint run

uninstall:
	sudo rm -f "$(DESTDIR)$(BINPREFIX)/$(TARGET)"
