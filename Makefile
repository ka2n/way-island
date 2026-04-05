BINARY := way-island
GOFLAGS := -tags=gtk4
RENDER_OUTPUT_DIR ?= .tmp-render
SNAPSHOT ?=
SNAPSHOT_TEST_PATTERN := TestGTKSnapshot
ifneq ($(strip $(SNAPSHOT)),)
SNAPSHOT_TEST_PATTERN := TestGTKSnapshot$(SNAPSHOT)
endif

.PHONY: build run test test-snapshot test-snapshot-update test-snapshot-clean vet lint check clean install rvinspect-build rvinspect-run

build:
	nix develop -c go build $(GOFLAGS) -o $(BINARY) .

run: build
	./$(BINARY)

test:
	go test ./...

test-snapshot:
	mkdir -p $(RENDER_OUTPUT_DIR)
	nix develop -c bash -lc 'tmpdir=$$(mktemp -d); pid=""; trap "rm -rf $$tmpdir; test -n \"$$pid\" && kill $$pid >/dev/null 2>&1 || true" EXIT; export XDG_CONFIG_HOME=$$tmpdir; export GSK_RENDERER=cairo; Xvfb :99 -screen 0 1280x1024x24 >/tmp/way-island-xvfb.log 2>&1 & pid=$$!; export DISPLAY=:99; WAY_ISLAND_RENDER_TEST_OUTPUT_DIR="$(PWD)/$(RENDER_OUTPUT_DIR)" go test -tags=gtk4 -run "$(SNAPSHOT_TEST_PATTERN)" ./...'

test-snapshot-update:
	mkdir -p $(RENDER_OUTPUT_DIR)
	nix develop -c bash -lc 'tmpdir=$$(mktemp -d); pid=""; trap "rm -rf $$tmpdir; test -n \"$$pid\" && kill $$pid >/dev/null 2>&1 || true" EXIT; export XDG_CONFIG_HOME=$$tmpdir; export GSK_RENDERER=cairo; export WAY_ISLAND_ACCEPT_SNAPSHOTS=1; Xvfb :99 -screen 0 1280x1024x24 >/tmp/way-island-xvfb.log 2>&1 & pid=$$!; export DISPLAY=:99; WAY_ISLAND_RENDER_TEST_OUTPUT_DIR="$(PWD)/$(RENDER_OUTPUT_DIR)" go test -tags=gtk4 -run "$(SNAPSHOT_TEST_PATTERN)" ./...'

test-snapshot-clean:
	rm -rf $(RENDER_OUTPUT_DIR)

vet:
	nix develop -c go vet $(GOFLAGS) ./...

lint: vet

check: vet test build

clean:
	rm -f $(BINARY) coverage.out

install:
	-nix profile remove way-island >/dev/null 2>&1
	nix profile install . --no-write-lock-file
	-systemctl --user daemon-reload >/dev/null 2>&1
	@printf '%s\n' 'Installed way-island to the current nix profile.'
	@printf '%s\n' 'To enable the user service: systemctl --user enable --now way-island.service'

rvinspect-build:
	nix develop --no-write-lock-file -c bash -lc 'cd tools/rvinspect && zig build'

rvinspect-run:
	nix develop --no-write-lock-file -c bash -lc 'cd tools/rvinspect && zig build run -- $(ARGS)'
