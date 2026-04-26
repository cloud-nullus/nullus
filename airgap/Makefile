# Nullus 에어갭 배포 Makefile
# 사용법: make help
# 전제조건: docker, kind, kubectl, helm 설치 필요

SHELL := /usr/bin/env bash
.DEFAULT_GOAL := help

# ── 경로 ─────────────────────────────────────────────────────
ROOT_DIR  := $(shell cd "$(dir $(abspath $(lastword $(MAKEFILE_LIST))))/.." && pwd)
AIRGAP    := $(ROOT_DIR)/airgap
SCRIPTS   := $(AIRGAP)/scripts

# ── 설정값 (환경변수로 오버라이드 가능) ──────────────────────
CLUSTER_NAME   ?= nullus-airgap
REGISTRY_NAME  ?= kind-registry
REGISTRY_PORT  ?= 5001
SKIP_LOAD      ?= 0
SKIP_VERIFY    ?= 0

export CLUSTER_NAME REGISTRY_NAME REGISTRY_PORT SKIP_LOAD SKIP_VERIFY

# ── 타겟 목록 ─────────────────────────────────────────────────

.PHONY: help
help: ## 사용 가능한 타겟 목록 출력
	@echo ""
	@echo "Nullus 에어갭 배포 도구"
	@echo "────────────────────────────────────────────"
	@echo "사용법: make <target>"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "환경변수:"
	@echo "  CLUSTER_NAME=$(CLUSTER_NAME)"
	@echo "  REGISTRY_NAME=$(REGISTRY_NAME)"
	@echo "  REGISTRY_PORT=$(REGISTRY_PORT)"
	@echo "  SKIP_LOAD=$(SKIP_LOAD)"
	@echo "  SKIP_VERIFY=$(SKIP_VERIFY)"
	@echo ""

.PHONY: images-regen
images-regen: ## 이미지 목록 자동 재생성 (helm template 기반, images.txt 덮어쓰기)
	@echo "==> images.txt 재생성"
	@bash $(SCRIPTS)/00-generate-images.sh

.PHONY: images-check
images-check: ## CI/PR 가드: 차트와 images.txt 드리프트 검사 (drift 시 exit 1)
	@echo "==> images.txt 드리프트 검사"
	@MODE=check bash $(SCRIPTS)/00-generate-images.sh

.PHONY: bundle
bundle: ## 이미지 번들 생성 (01-pull + 02-save)
	@echo "==> 번들 생성: pull + save"
	@bash $(SCRIPTS)/01-pull-images.sh
	@bash $(SCRIPTS)/02-save-bundle.sh

.PHONY: load
load: ## 번들 이미지를 docker load (03-load-bundle.sh)
	@echo "==> 번들 이미지 docker load"
	@bash $(SCRIPTS)/03-load-bundle.sh

.PHONY: registry
registry: ## [Agent 2] 로컬 레지스트리(registry:2) 컨테이너 기동
	@echo "==> 로컬 레지스트리 기동: $(REGISTRY_NAME):$(REGISTRY_PORT)"
	@bash $(SCRIPTS)/10-setup-registry.sh

.PHONY: cluster
cluster: ## [Agent 2] kind 클러스터 생성 및 레지스트리 연결
	@echo "==> kind 클러스터 생성: $(CLUSTER_NAME)"
	@bash $(SCRIPTS)/11-create-cluster.sh

.PHONY: push
push: ## [Agent 2] 이미지를 로컬 레지스트리로 리태그 후 푸시
	@echo "==> 이미지 푸시 → localhost:$(REGISTRY_PORT)"
	@bash $(SCRIPTS)/12-push-to-registry.sh

.PHONY: charts
charts: ## 차트 번들 생성 (20-bundle-charts.sh, 온라인 전용)
	@echo "==> helm 의존성 번들"
	@bash $(SCRIPTS)/20-bundle-charts.sh

.PHONY: install
install: ## Helm 차트로 Nullus 설치 (21-install-nullus.sh)
	@echo "==> Nullus helm install"
	@bash $(SCRIPTS)/21-install-nullus.sh

.PHONY: verify
verify: ## [Agent 2] 클러스터 상태 검증 (노드/파드/레지스트리/이미지)
	@echo "==> 클러스터 검증: $(CLUSTER_NAME)"
	@bash $(SCRIPTS)/99-verify.sh

.PHONY: all
all: ## 전체 부트스트랩 실행 (bootstrap.sh)
	@echo "==> 전체 에어갭 부트스트랩 시작"
	@bash $(SCRIPTS)/bootstrap.sh

.PHONY: clean
clean: ## kind 클러스터 삭제 및 레지스트리 컨테이너 제거
	@echo "==> 클린업: 클러스터 '$(CLUSTER_NAME)' 삭제 + 레지스트리 컨테이너 제거"
	@if kind get clusters 2>/dev/null | grep -q '^$(CLUSTER_NAME)$$'; then \
		kind delete cluster --name $(CLUSTER_NAME); \
		echo "  클러스터 삭제 완료"; \
	else \
		echo "  클러스터 '$(CLUSTER_NAME)' 없음 — 건너뜀"; \
	fi
	@if docker inspect $(REGISTRY_NAME) &>/dev/null; then \
		docker rm -f $(REGISTRY_NAME); \
		echo "  레지스트리 컨테이너 제거 완료"; \
	else \
		echo "  레지스트리 컨테이너 '$(REGISTRY_NAME)' 없음 — 건너뜀"; \
	fi
