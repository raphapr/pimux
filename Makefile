BIN ?= $(HOME)/.local/bin/pimux
EXT_DIR ?= $(HOME)/.pi/agent/extensions

.PHONY: build install test vet fmt install-extension clean

build:
	go build -o pimux .

install:
	go build -o $(BIN) .
	@echo "installed $(BIN)"

install-extension:
	cp extension/pimux-reporter.ts $(EXT_DIR)/pimux-reporter.ts
	@echo "installed $(EXT_DIR)/pimux-reporter.ts"

test:
	go test ./...
	cd extension && node --test --experimental-strip-types pimux-reporter.test.mjs

vet:
	go vet ./...

fmt:
	gofmt -w .

clean:
	rm -f pimux
