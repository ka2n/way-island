BINARY := way-island
GOFLAGS := -tags=gtk4
RENDER_OUTPUT_DIR ?= .tmp-render

.PHONY: build run test test-snapshot vet lint check clean install

build:
	nix develop -c go build $(GOFLAGS) -o $(BINARY) .

run: build
	./$(BINARY)

test:
	go test ./...

test-snapshot:
	mkdir -p $(RENDER_OUTPUT_DIR)
	nix develop -c bash -lc 'tmpdir=$$(mktemp -d); pid=""; trap "rm -rf $$tmpdir; test -n \"$$pid\" && kill $$pid >/dev/null 2>&1 || true" EXIT; export XDG_CONFIG_HOME=$$tmpdir; export GSK_RENDERER=cairo; Xvfb :99 -screen 0 1280x1024x24 >/tmp/way-island-xvfb.log 2>&1 & pid=$$!; export DISPLAY=:99; WAY_ISLAND_RENDER_TEST_OUTPUT_DIR="$(PWD)/$(RENDER_OUTPUT_DIR)" go test -tags=gtk4 -run TestGTKSnapshot ./...'

vet:
	nix develop -c go vet $(GOFLAGS) ./...

lint: vet

check: vet test build

clean:
	rm -f $(BINARY) coverage.out

install: build
	install -Dm755 $(BINARY) $(DESTDIR)$(PREFIX)/bin/$(BINARY)

PREFIX ?= /usr/local
DESTDIR ?=
