PREFIX ?= /usr/local

build:
	go build -o mgt main.go

install: build
	mkdir -p $(PREFIX)/bin
	cp mgt $(PREFIX)/bin/mgt

uninstall:
	rm -f $(PREFIX)/bin/mgt

test:
	go test ./...

clean:
	rm -f mgt
