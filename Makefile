# https://www.gnu.org/prep/standards/html_node/Directory-Variables.html#Directory-Variables
PREFIX    ?= /usr/local
BINPREFIX ?= $(PREFIX)/bin
DISTDIR   := dist
TARGET    := gog
PLATFORMS := darwin freebsd linux

.PHONY: $(PLATFORMS) $(TARGET) all clean coverage coverage-html install lint release test uninstall

all: $(TARGET)

$(PLATFORMS):
	GOARCH=amd64 GOOS=$@ go build -o "$(DISTDIR)/$(TARGET)-$@-amd64"

$(TARGET):
	go build -o $@

clean:
	go clean
	rm -f $(DISTDIR)/$(TARGET)*
	rm -f coverage.out coverage.html

install: $(TARGET)
	sudo mkdir -p "$(DESTDIR)$(BINPREFIX)"
	sudo cp -pf $(TARGET) "$(DESTDIR)$(BINPREFIX)/"

release: clean $(PLATFORMS)

test:
	go test -v -race -coverprofile=coverage.out ./...

coverage: test
	go tool cover -func=coverage.out

coverage-html: test
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

lint:
	golangci-lint run

uninstall:
	rm -f "$(DESTDIR)$(BINPREFIX)/$(TARGET)"
