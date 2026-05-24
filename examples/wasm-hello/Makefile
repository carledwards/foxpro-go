# wasm-hello — build + serve foxpro-go in the browser.
#
#   make           — build wasm, copy wasm_exec.js, serve on :8765
#   make build     — just rebuild sim.wasm
#   make serve     — serve the existing web/ on :8765
#   make clean     — remove build artifacts

PORT   ?= 8765
PKG    := github.com/carledwards/foxpro-go/examples/wasm-hello
WEB    := web
WASM   := $(WEB)/sim.wasm
EXEC   := $(WEB)/wasm_exec.js
FOXPRO := $(WEB)/foxpro.js

# Locate wasm_exec.js inside the active Go toolchain. Path is
# $(GOROOT)/lib/wasm/wasm_exec.js on Go 1.24+ and
# $(GOROOT)/misc/wasm/wasm_exec.js on older versions — try both.
GOROOT := $(shell go env GOROOT)
EXEC_SRC := $(firstword $(wildcard $(GOROOT)/lib/wasm/wasm_exec.js $(GOROOT)/misc/wasm/wasm_exec.js))

# foxpro.js is the shared canvas renderer + input bridge that lives at
# the repo root under wasm/. Copied into web/ so the static deploy is
# self-contained — external consumers (other repos) pull it from the
# Go module cache instead, but we're in-tree.
FOXPRO_SRC := ../../wasm/foxpro.js

.PHONY: all build serve clean wasm

all: build serve

# `go build` does its own change detection and is fast enough that
# always invoking it is simpler than mirroring Go's import graph here.
# That way edits in foxpro-go/* or foxpro-go/wasm/* trigger a rebuild
# without us having to enumerate dependencies.
build: wasm $(EXEC) $(FOXPRO)

wasm:
	@mkdir -p $(WEB)
	GOOS=js GOARCH=wasm go build -o $(WASM) $(PKG)
	@ls -lh $(WASM) | awk '{print "  built " $$NF " (" $$5 ")"}'

$(EXEC):
	@mkdir -p $(WEB)
	@if [ -z "$(EXEC_SRC)" ]; then \
	  echo "ERROR: wasm_exec.js not found under $(GOROOT)/{lib,misc}/wasm/"; \
	  exit 1; \
	fi
	cp $(EXEC_SRC) $@

# Always re-copy foxpro.js (cheap, no PHONY-rebuild target needed).
# .PHONY would cause a rebuild every invocation; this rule only re-runs
# when the source file is newer than the copy.
$(FOXPRO): $(FOXPRO_SRC)
	@mkdir -p $(WEB)
	cp $(FOXPRO_SRC) $@

serve:
	@echo "serving $(WEB)/ at http://localhost:$(PORT)/"
	@cd $(WEB) && python3 -m http.server $(PORT)

clean:
	rm -f $(WASM) $(EXEC) $(FOXPRO)
