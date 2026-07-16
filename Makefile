# GrokProxy Makefile
#
# 目标平台：linux / darwin (macOS) / windows，默认 amd64 与 arm64 双架构。
# 交叉编译通过 Wails 的 -platform 参数完成，产物写入 build/bin。

WAILS      ?= wails
PNPM       ?= pnpm
GO         ?= go

# 平台与架构矩阵：用空格分隔多个 target，每个 target 形如 os/arch。
PLATFORMS   ?= linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64

# 版本号：优先读 VERSION 文件，回退到 git describe。
VERSION    := $(shell cat VERSION 2>/dev/null | tr -d '[:space:]')
ifeq ($(strip $(VERSION)),)
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
endif

LDFLAGS    := -s -w -X main.version=$(VERSION)

BIN_DIR     = build/bin
empty       :=
space       := $(empty) $(empty)
comma       := ,

.PHONY: help install test lint dev build build-all release clean help-platforms

help: ## 显示帮助
	@awk 'BEGIN {FS = ":.*##"; printf "可用目标:\n"} /^[a-zA-Z_-]+:.*##/ {printf "  %-16s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

install: ## 安装 Go 与前端依赖
	$(GO) mod download
	$(PNPM) --dir frontend install --frozen-lockfile

test: ## 运行 Go 测试
	$(GO) test -race ./...

lint: ## 运行静态检查
	$(GO) vet ./...
	$(PNPM) --dir frontend typecheck
	$(PNPM) --dir frontend lint

dev: ## 以开发模式运行桌面程序
	$(WAILS) dev

build: ## 为当前平台构建
	$(WAILS) build -clean -trimpath -ldflags "$(LDFLAGS)"

build-all: ## 为所有目标平台交叉编译
	$(WAILS) build -clean -trimpath -ldflags "$(LDFLAGS)" -platform "$(subst $(space),$(comma),$(PLATFORMS))"

release: build-all ## 构建全部平台产物，用于发布

clean: ## 清理构建产物
	@rm -rf $(BIN_DIR)/*
	@rm -rf frontend/dist
	@rm -rf frontend/wailsjs

help-platforms: ## 列出默认目标平台
	@echo "默认目标平台: $(PLATFORMS)"