PREFIX    ?= /usr/local
BINPREFIX ?= $(PREFIX)/bin
TARGET = gog

.PHONY: all clean install uninstall

gog: clean
	go get
	go build

all: 
	@echo "Run make install"

clean:
	    rm -f $(TARGET)

install: gog
	sudo mkdir -p "$(DESTDIR)$(BINPREFIX)"
	sudo cp -pf gog "$(DESTDIR)$(BINPREFIX)/"

uninstall:
	rm -f "$(DESTDIR)$(BINPREFIX)/$(TARGET)"
