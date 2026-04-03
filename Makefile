BINARY := way-island
GOFLAGS := -tags=gtk4

.PHONY: build run test vet lint check clean install

build:
	go build $(GOFLAGS) -o $(BINARY) .

run: build
	./$(BINARY)

test:
	go test $(GOFLAGS) ./...

vet:
	go vet $(GOFLAGS) ./...

lint: vet

check: vet test build

clean:
	rm -f $(BINARY) coverage.out

install: build
	install -Dm755 $(BINARY) $(DESTDIR)$(PREFIX)/bin/$(BINARY)

PREFIX ?= /usr/local
DESTDIR ?=
