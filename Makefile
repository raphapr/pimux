BIN ?= $(HOME)/.local/bin/pimux

.PHONY: build install test vet fmt clean

build:
	go build -o pimux .

install:
	go build -o $(BIN) .
	@echo "installed $(BIN)"
	@echo "run 'pimux install-extension' to install the reporter"

test:
	go test ./...
	cd extension && node --test --experimental-strip-types pimux-reporter.test.mjs

vet:
	go vet ./...

fmt:
	gofmt -w .

clean:
	rm -f pimux
