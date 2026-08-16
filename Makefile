# https://www.gnu.org/prep/standards/html_node/Directory-Variables.html#Directory-Variables
PREFIX    ?= /usr/local
BINPREFIX ?= $(PREFIX)/bin
DISTDIR   := dist
TARGET    := gog
# The flags .goreleaser.yaml builds a tag with, so a binary built here is the
# binary a release ships rather than an unstripped one carrying local paths.
LDFLAGS   := -s -w
PLATFORMS := darwin freebsd linux

.PHONY: $(PLATFORMS) $(TARGET) all build clean coverage coverage-html fmt install lint release test uninstall

all: $(TARGET)

build: $(TARGET)

$(PLATFORMS):
	GOARCH=amd64 GOOS=$@ go build -ldflags="$(LDFLAGS)" -trimpath -o "$(DISTDIR)/$(TARGET)-$@-amd64"

$(TARGET):
	go build -ldflags="$(LDFLAGS)" -trimpath -o $@

clean:
	go clean
	rm -f $(DISTDIR)/$(TARGET)*
	rm -f coverage.txt coverage.html

install: $(TARGET)
	sudo mkdir -p "$(DESTDIR)$(BINPREFIX)"
	sudo cp -pf $(TARGET) "$(DESTDIR)$(BINPREFIX)/"

release: clean $(PLATFORMS)

test:
	go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...

coverage: test
	go tool cover -func=coverage.txt

coverage-html: test
	go tool cover -html=coverage.txt -o coverage.html
	@echo "Coverage report generated: coverage.html"

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
