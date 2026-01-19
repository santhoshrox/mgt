.PHONY: build install test clean

build:
	go build -o mgt main.go

install:
	go install .

test:
	go test ./...

clean:
	rm -f mgt
