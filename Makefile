# https://www.gnu.org/prep/standards/html_node/Directory-Variables.html#Directory-Variables
PREFIX    ?= /usr/local
BINPREFIX ?= $(PREFIX)/bin
DISTDIR := dist
TARGET := gog
PKGS := $(shell go list ./... | grep -v /vendor)
PLATFORMS := darwin freebsd linux

.PHONY: $(PLATFORMS) $(TARGET) all clean deps install release test uninstall

all: $(TARGET)

$(PLATFORMS):
	GOARCH=amd64 GOOS=$@ go build -o "$(DISTDIR)/$@-amd64/$(TARGET)"

$(TARGET): deps
	go build -o $@

clean:
	go clean
	rm -rf $(DISTDIR)

deps:
	go get

install: $(TARGET)
	sudo mkdir -p "$(DESTDIR)$(BINPREFIX)"
	sudo cp -pf $(TARGET) "$(DESTDIR)$(BINPREFIX)/"

release: clean $(PLATFORMS)

test:
	go test -v $(PKGS)

uninstall:
	rm -f "$(DESTDIR)$(BINPREFIX)/$(TARGET)"
