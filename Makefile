# https://www.gnu.org/prep/standards/html_node/Directory-Variables.html#Directory-Variables
PREFIX    ?= /usr/local
BINPREFIX ?= $(PREFIX)/bin
DISTDIR   := dist
TARGET    := gog
PLATFORMS := darwin freebsd linux

.PHONY: $(PLATFORMS) $(TARGET) all build clean coverage coverage-html fmt install lint release test uninstall

all: $(TARGET)

build: $(TARGET)

$(PLATFORMS):
	GOARCH=amd64 GOOS=$@ go build -o "$(DISTDIR)/$(TARGET)-$@-amd64"

$(TARGET):
	go build -o $@

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

lint:
	golangci-lint run

uninstall:
	rm -f "$(DESTDIR)$(BINPREFIX)/$(TARGET)"
