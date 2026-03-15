# Backend API Implementation & Frontend Integration Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Go 백엔드 API를 프론트엔드 API 계약에 맞춰 완성하고, 프론트엔드 mock 데이터를 실제 API 호출로 교체하여 풀스택 연동 달성

**Architecture:** 백엔드가 프론트엔드 API 계약에 적응 (라우트 경로 + 응답 포맷 정렬). 기존 Clean Architecture 레이어(domain/usecase/port/adapter) 유지. 프론트엔드 TanStack Query 훅에서 mock 데이터를 Axios 호출로 교체.

**Tech Stack:** Go 1.24+ (Echo v4, pgx/v5, client-go v0.35.2), React 19 (TypeScript, TanStack Query, Axios, Zustand)

---

## Execution Summary

### Phase 1: Foundation (완료)
- Task 1: Go 의존성 추가 (client-go, gorilla/websocket) ✅
- Task 2: AES-256-GCM 암호화 유틸 (pkg/crypto/) ✅ (6 tests pass)

### Phase 2: Backend Route Alignment + Missing Endpoints (진행중)
- CORS 미들웨어 추가
- 모든 핸들러 라우트를 프론트엔드 API 계약에 맞춤
- 응답 포맷: `{data: ...}` → 직접 객체 / `{items: [], total: N}`
- client-go 클러스터 검증 엔드포인트
- Kubeconfig AES-256-GCM 암호화 저장
- 누락 엔드포인트 추가 (멤버 관리, 알림 CRUD 등)

### Phase 3: OpenAPI 3.0 Spec (진행중)
- api/openapi.yaml 생성

### Phase 4: Frontend API Integration (진행중)
- Vite 프록시 설정 (localhost:8090)
- 4개 API 파일의 모든 TanStack Query 훅을 실제 Axios 호출로 교체
- Mock 데이터 완전 제거

### Phase 5: Verification
- Go 빌드 + 테스트
- Frontend 빌드 + 테스트
- 통합 연동 확인

---

## Route Alignment Map

| Frontend expects | Backend current | Action |
|-----------------|----------------|--------|
| `/admin/clusters` | `/clusters` | Route prefix 변경 |
| `/admin/organization` | `/orgs/:orgId` | 새 핸들러 추가 |
| `/stacks/templates` | `/templates` | Route prefix 변경 |
| `/stacks/compatibility` | `/compatibility/matrix` | Route 변경 |
| `/cicd/templates` | `/cicd-templates` | Route 변경 |
| `/cicd/pipelines` | `/pipelines` | Route prefix 변경 |
| `/observability/dashboard` | `/monitoring/dashboard` | Route prefix 변경 |
| `/observability/alert-rules` | `/alerts/rules` | Route 변경 |
