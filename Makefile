# Makefile: Milestone 0.2 (Tooling & CI), Track D
#
# Dijalankan dari root repo, di Git Bash (Windows) atau bash (runner CI).
# Target utama: lint, test, build, zip (opsional). Daftar lengkap: make help
#
# Versi golangci-lint di-pin SATU kali, di bawah ini (GOLANGCI_LINT_VERSION).
# Workflow CI membaca nilai ini lewat: make -s print-golangci-lint-version

SHELL := bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

# ---------- Konfigurasi ----------

GOLANGCI_LINT_VERSION ?= v2.14.0
GOLANGCI_LINT_PKG := github.com/golangci/golangci-lint/v2/cmd/golangci-lint
GOLANGCI_CONFIG ?= $(CURDIR)/.golangci.yml
SKIP_LINT_VERSION_CHECK ?=

GO ?= go
GOLANGCI_LINT ?= golangci-lint
PNPM ?= pnpm
WAILS ?= wails
PYTHON ?= python

SERVER_DIR := server
DESKTOP_DIR := desktop
FRONTEND_DIR := $(DESKTOP_DIR)/frontend
DIST_DIR := $(FRONTEND_DIR)/dist
GO_MODULES := $(SERVER_DIR) $(DESKTOP_DIR)

# Script npm di desktop/frontend/package.json
FRONTEND_CHECK_SCRIPT ?= check
FRONTEND_BUILD_SCRIPT ?= build

# Keluaran make zip: di LUAR repo agar tidak mengotori working tree
ZIP_OUT ?= ../$(notdir $(CURDIR))-source.zip

.PHONY: help lint lint-go lint-frontend test test-go build build-server \
        build-frontend build-desktop deps tools doctor ensure-dist zip \
        check-golangci-lint print-golangci-lint-version

# ---------- Bantuan ----------

help: ## Tampilkan daftar target
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# ---------- Lint ----------

lint: lint-go lint-frontend ## Jalankan semua lint (golangci-lint + svelte-check)

lint-go: ensure-dist check-golangci-lint ## golangci-lint untuk modul server dan desktop
	@rc=0; \
	for m in $(GO_MODULES); do \
		echo ">> golangci-lint: $$m"; \
		( cd "$$m" && $(GOLANGCI_LINT) run --config "$(GOLANGCI_CONFIG)" ./... ) || rc=1; \
	done; \
	exit $$rc

lint-frontend: ## svelte-check untuk frontend desktop
	$(PNPM) --dir $(FRONTEND_DIR) run $(FRONTEND_CHECK_SCRIPT)

# ---------- Test ----------

test: test-go ## Jalankan semua test (go test server dan desktop)

test-go: ensure-dist ## go test ./... untuk modul server dan desktop
	@rc=0; \
	for m in $(GO_MODULES); do \
		echo ">> go test: $$m"; \
		( cd "$$m" && $(GO) test ./... ) || rc=1; \
	done; \
	exit $$rc

# ---------- Build ----------

build: build-server build-desktop ## Build server (cek compile) dan aplikasi desktop

build-server: ## Kompilasi semua package server (tanpa artefak)
	@cd $(SERVER_DIR) && $(GO) build ./...

build-frontend: ## Build frontend desktop (vite), lalu pulihkan dist/.gitkeep
	$(PNPM) --dir $(FRONTEND_DIR) run $(FRONTEND_BUILD_SCRIPT)
	@$(MAKE) --no-print-directory ensure-dist

build-desktop: build-frontend ## wails build tanpa build frontend ulang (-s)
	cd $(DESKTOP_DIR) && $(WAILS) build -s

# ---------- Dependensi dan tool ----------

deps: ## pnpm install --frozen-lockfile (tidak mengubah pnpm-lock.yaml)
	$(PNPM) install --frozen-lockfile

tools: ## Pasang golangci-lint versi pin ke GOBIN/GOPATH (tanpa Administrator)
	GOWORK=off $(GO) install $(GOLANGCI_LINT_PKG)@$(GOLANGCI_LINT_VERSION)
	@echo "Terpasang di GOBIN atau $$($(GO) env GOPATH)/bin. Pastikan folder itu ada di PATH."

doctor: ## Tampilkan versi tool yang terpasang
	@show() { name="$$1"; shift; \
		if command -v "$$1" >/dev/null 2>&1; then \
			printf '%-16s %s\n' "$$name" "$$("$$@" 2>&1 | head -n1)"; \
		else \
			printf '%-16s %s\n' "$$name" "TIDAK TERPASANG"; \
		fi; }; \
	show make make --version; \
	show go $(GO) version; \
	show node node --version; \
	show pnpm $(PNPM) --version; \
	show wails $(WAILS) version; \
	show golangci-lint $(GOLANGCI_LINT) --version; \
	show python $(PYTHON) --version; \
	echo "pin golangci-lint: $(GOLANGCI_LINT_VERSION)"

check-golangci-lint:
	@command -v $(GOLANGCI_LINT) >/dev/null 2>&1 || { \
		echo "ERROR: golangci-lint tidak ditemukan di PATH."; \
		echo "       Jalankan 'make tools', lalu pastikan folder bin GOPATH ada di PATH."; \
		exit 1; }
	@if [ -z "$(SKIP_LINT_VERSION_CHECK)" ]; then \
		have="$$($(GOLANGCI_LINT) --version 2>&1 | sed -n 's/.*version \([0-9][0-9.]*\).*/\1/p' | head -n1)"; \
		want="$(patsubst v%,%,$(GOLANGCI_LINT_VERSION))"; \
		if [ "$$have" != "$$want" ]; then \
			echo "ERROR: golangci-lint terpasang '$$have', Makefile mem-pin '$$want'."; \
			echo "       Jalankan 'make tools' (atau SKIP_LINT_VERSION_CHECK=1 untuk melewati)."; \
			exit 1; \
		fi; \
	fi

print-golangci-lint-version:
	@echo $(GOLANGCI_LINT_VERSION)

# ---------- Penjaga embed ----------
# desktop memakai go:embed pada frontend/dist. 'vite build' dan 'wails dev'
# menghapus dist/.gitkeep, sehingga go vet/test/lint gagal ("no matching files")
# dan git melihat berkas terhapus. Target ini memastikan folder dan .gitkeep ada.

ensure-dist:
	@mkdir -p $(DIST_DIR)
	@touch $(DIST_DIR)/.gitkeep

# ---------- ZIP (opsional, untuk dibawa ke chat room AI) ----------
# Berkas yang tercakup: yang dilacak git + yang belum dilacak tapi tidak di-ignore.
# Dikecualikan: .git, node_modules, build/, *.exe, .env (lihat tools/zip_source.py).

zip: ## (opsional) ZIP source tanpa .git, node_modules, build/, *.exe, .env
	git ls-files -z --cached --others --exclude-standard | $(PYTHON) tools/zip_source.py "$(ZIP_OUT)"
