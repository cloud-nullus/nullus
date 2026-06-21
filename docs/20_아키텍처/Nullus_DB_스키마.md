# Nullus Platform PostgreSQL 데이터베이스 스키마 설계

**작성일**: 2026-03-14
**버전**: 1.0
**기반 문서**: nullus_PRD_1.3.md, Nullus_시스템_아키텍처.md, Nullus_기능목록.md, Nullus_기능분해도.csv, CLAUDE.md
**대상 독자**: 백엔드 엔지니어, DBA, 아키텍트

---

## 목차

1. [설계 원칙](#1-설계-원칙)
2. [Bounded Context별 테이블 분류](#2-bounded-context별-테이블-분류)
3. [ERD (텍스트 기반 관계도)](#3-erd-텍스트-기반-관계도)
4. [모듈 간 참조 규칙](#4-모듈-간-참조-규칙)
5. [공통 컨벤션](#5-공통-컨벤션)
6. [Context 1: Organization (조직 관리)](#6-context-1-organization-조직-관리)
7. [Context 2: Cluster (클러스터 관리)](#7-context-2-cluster-클러스터-관리)
8. [Context 3: Stack (DevSecOps 스택 관리)](#8-context-3-stack-devsecops-스택-관리)
9. [Context 4: CI/CD (파이프라인 관리)](#9-context-4-cicd-파이프라인-관리)
10. [Context 5: Observability (관측성)](#10-context-5-observability-관측성)
11. [Context 6: Auth (인증/인가)](#11-context-6-auth-인증인가)
12. [공통 테이블: 감사 로그](#12-공통-테이블-감사-로그)
13. [Kubeconfig 암호화 저장 방식](#13-kubeconfig-암호화-저장-방식)
14. [JSONB 활용 전략](#14-jsonb-활용-전략)
15. [마이그레이션 전략](#15-마이그레이션-전략)
16. [이력 관리 테이블 설계](#16-이력-관리-테이블-설계)

---

## 1. 설계 원칙

### 1.1 DDD + Modular Monolith 기반

CLAUDE.md에 정의된 아키텍처 원칙에 따라 데이터베이스를 설계한다.

- **모듈별 테이블 소유**: 각 Bounded Context(모듈)는 자신의 테이블만 직접 조회한다. 다른 모듈의 테이블을 직접 JOIN하지 않는다.
- **모듈 간 참조**: FK 대신 ID 값만 저장하고, 필요 시 API 호출 또는 도메인 이벤트로 데이터를 동기화한다.
- **Aggregate Root 단위 Repository**: 각 Aggregate Root(예: Organization, Stack, Pipeline)에 대해 하나의 Repository를 정의한다.

### 1.2 네이밍 규칙

| 항목 | 규칙 | 예시 |
|------|------|------|
| 테이블명 | snake_case, 복수형 | `organizations`, `stack_configs` |
| 컬럼명 | snake_case | `created_at`, `org_id` |
| PK | `id` (UUID v7) | `id UUID PRIMARY KEY` |
| FK | `{참조테이블_단수}_id` | `org_id`, `user_id` |
| 타임스탬프 | `TIMESTAMPTZ` 사용 | `created_at TIMESTAMPTZ` |
| 소프트 삭제 | `deleted_at` 컬럼 | `deleted_at TIMESTAMPTZ` |
| JSONB | `_json` 또는 `_config` 접미사 | `config_json`, `tools_config` |
| 암호화 컬럼 | `_encrypted` 접미사 | `kubeconfig_encrypted` |

### 1.3 공통 제약조건

- 모든 테이블에 `created_at`, `updated_at` 컬럼 포함
- PK는 UUID v7 (시간 정렬 가능) 사용
- 소프트 삭제가 필요한 테이블에 `deleted_at TIMESTAMPTZ` 컬럼 추가
- JSONB 컬럼에는 GIN 인덱스 적용

---

## 2. Bounded Context별 테이블 분류

CLAUDE.md의 Bounded Context 정의를 따른다.

| Context | 모듈 경로 | 소유 테이블 | Aggregate Root |
|---------|----------|------------|----------------|
| **Organization** | `internal/admin/` | `organizations`, `org_members`, `org_cluster_access`, `invite_links` | Organization |
| **Cluster** | `internal/admin/` | `clusters` | Cluster |
| **Stack** | `internal/stack/` | `stack_configs`, `stack_config_versions`, `stack_helm_step_configs`, `deployments`, `deployment_logs`, `deployment_steps`, `golden_path_templates`, `golden_path_template_tools`, `compatibility_matrices`, `compatibility_tools` | Stack, GoldenPathTemplate, CompatibilityMatrix |
| **CI/CD** | `internal/cicd/` | `pipeline_templates`, `pipeline_template_versions`, `pipeline_configs`, `pipeline_deployments` | Pipeline |
| **Observability** | `internal/observability/` | `alert_configs`, `alert_history` | Alert |
| **Auth** | `internal/auth/` | `users`, `sessions`, `rbac_policies`, `menu_permissions` | User, Session |
| **공통** | `internal/shared/` | `audit_logs` | AuditLog |

---

## 3. ERD (텍스트 기반 관계도)

```
                            ┌──────────────────┐
                            │      users       │
                            │   (Auth Context) │
                            │                  │
                            │  id (PK)         │
                            │  email           │
                            │  password_hash   │
                            │  display_name    │
                            │  is_active       │
                            └────────┬─────────┘
                                     │
              ┌──────────────────────┼──────────────────────┐
              │                      │                      │
              ▼                      ▼                      ▼
   ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐
   │   org_members    │  │    sessions      │  │   audit_logs     │
   │                  │  │  (Auth Context)  │  │   (공통)          │
   │  org_id (FK)     │  │                  │  │                  │
   │  user_id (FK)    │  │  user_id (FK)    │  │  actor_id        │
   │  role            │  │  token_hash      │  │  action          │
   └──────┬───────────┘  │  expires_at      │  │  resource_type   │
          │              └──────────────────┘  │  resource_id     │
          │                                    └──────────────────┘
          ▼
   ┌──────────────────┐        ┌──────────────────────┐
   │  organizations   │───────>│  org_cluster_access  │
   │ (Org Context)    │  1:N   │                      │
   │                  │        │  org_id (FK)         │
   │  id (PK)         │        │  cluster_id          │
   │  name            │        └──────────┬───────────┘
   │  slug (UNIQUE)   │                   │ (느슨한 참조)
   │  status          │                   ▼
   └──────────────────┘        ┌──────────────────────┐
                               │     clusters         │
          ┌───────────────────>│  (Cluster Context)   │
          │                    │                      │
          │                    │  id (PK)             │
          │                    │  org_id              │
          │                    │  name                │
          │                    │  type                │
          │                    │  kubeconfig_encrypted│
          │                    │  status              │
          │                    └──────────┬───────────┘
          │                               │
          │                               │ (느슨한 참조: cluster_id)
          │                               ▼
          │                    ┌──────────────────────────┐
          │                    │    stack_configs         │
          │                    │  (Stack Context)         │
          │                    │                          │
          │                    │  id (PK)                 │
          │                    │  cluster_id              │
          │                    │  org_id                  │
          │                    │  config_json (JSONB)     │
          │                    │  status                  │
          │                    │  current_version         │
          │                    └─────┬──────────┬─────────┘
          │                          │          │
          │                    1:N   │          │ 1:N
          │                          ▼          ▼
          │       ┌────────────────────┐  ┌──────────────────────────┐
          │       │ stack_config_      │  │     deployments          │
          │       │   versions         │  │                          │
          │       │                    │  │  id (PK)                 │
          │       │  stack_config_id   │  │  stack_config_id (FK)    │
          │       │  version_number    │  │  type                    │
          │       │  config_snapshot   │  │  status                  │
          │       │  changed_by        │  │  started_by              │
          │       └────────────────────┘  └──────────┬───────────────┘
          │                                          │
          │                                    1:N   │
          │                                          ▼
          │                               ┌──────────────────────┐
          │                               │  deployment_logs     │
          │                               │                      │
          │                               │  deployment_id (FK)  │
          │                               │  step_name           │
          │                               │  level               │
          │                               │  message             │
          │                               └──────────────────────┘
          │
          │    ┌──────────────────────────┐     ┌──────────────────────────┐
          │    │   pipeline_configs       │────>│  pipeline_deployments    │
          │    │   (CI/CD Context)        │ 1:N │                          │
          │    │                          │     │  pipeline_config_id (FK) │
          │    │  id (PK)                 │     │  version                 │
          │    │  stack_config_id         │     │  status                  │
          │    │  template_id             │     │  k8s_objects (JSONB)     │
          │    │  params_json (JSONB)     │     │  deployed_by             │
          │    └──────────────────────────┘     └──────────────────────────┘
          │
          │    ┌──────────────────────────┐     ┌──────────────────────────┐
          │    │   alert_configs          │     │   alert_history          │
          │    │   (Obs Context)          │────>│                          │
          │    │                          │ 1:N │  alert_config_id (FK)    │
          │    │  stack_config_id         │     │  severity                │
          │    │  channel                 │     │  message                 │
          │    │  enabled                 │     │  fired_at                │
          │    └──────────────────────────┘     └──────────────────────────┘
          │
          │    ┌──────────────────────────┐
          │    │ golden_path_templates    │
          │    │ (Stack Context)          │
          │    │                          │
          │    │  id (PK)                 │
          │    │  name                    │
          │    │  tools_config (JSONB)    │
          │    │  resource_baseline       │
          │    └──────────────────────────┘
          │
          │    ┌──────────────────────────┐
          └────│ compatibility_matrices   │
               │ (Stack Context)          │
               │                          │
               │  id (PK)                 │
               │  name                    │
               │  status                  │
               │  kubernetes_versions     │
               │  tools (JSONB)           │
               └──────────────────────────┘
```

---

## 4. 모듈 간 참조 규칙

### 4.1 동일 Context 내: FK 직접 참조

같은 Bounded Context 내의 테이블은 FK로 직접 참조한다.

```
예: stack_configs → stack_config_versions (FK: stack_config_id)
예: pipeline_configs → pipeline_deployments (FK: pipeline_config_id)
예: organizations → org_members (FK: org_id)
```

### 4.2 Context 간: 느슨한 참조 (Loose Reference)

다른 Bounded Context의 테이블은 FK 제약조건 없이 ID 값만 저장한다.

| 참조하는 테이블 | 참조 컬럼 | 참조 대상 (다른 Context) | 참조 방식 |
|----------------|----------|------------------------|-----------|
| `clusters.org_id` | `org_id` | `organizations.id` (Org Context) | ID만 저장, FK 없음 |
| `stack_configs.cluster_id` | `cluster_id` | `clusters.id` (Cluster Context) | ID만 저장, FK 없음 |
| `stack_configs.org_id` | `org_id` | `organizations.id` (Org Context) | ID만 저장, FK 없음 |
| `pipeline_configs.stack_config_id` | `stack_config_id` | `stack_configs.id` (Stack Context) | ID만 저장, FK 없음 |
| `alert_configs.stack_config_id` | `stack_config_id` | `stack_configs.id` (Stack Context) | ID만 저장, FK 없음 |
| `deployments.started_by` | `started_by` | `users.id` (Auth Context) | ID만 저장, FK 없음 |
| `stack_config_versions.changed_by` | `changed_by` | `users.id` (Auth Context) | ID만 저장, FK 없음 |

### 4.3 도메인 이벤트 기반 동기화

모듈 간 데이터 일관성이 필요한 경우 도메인 이벤트를 사용한다.

| 이벤트 | 발행 Context | 구독 Context | 동작 |
|--------|-------------|-------------|------|
| `OrganizationDeleted` | Organization | Cluster, Stack | 관련 클러스터/스택 비활성화 |
| `ClusterDeleted` | Cluster | Stack | 해당 클러스터의 스택 상태를 `orphaned`로 변경 |
| `StackDeployed` | Stack | Observability | 모니터링 대시보드 자동 생성 |
| `PipelineDeployed` | CI/CD | Observability | 파이프라인 메트릭 수집 시작 |
| `UserDeactivated` | Auth | Organization | 해당 사용자의 org_members 비활성화 |

---

## 5. 공통 컨벤션

### 5.1 공통 타입 정의

```sql
-- UUID v7 생성 함수 (pgcrypto 확장 사용)
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 공통 상태 enum
CREATE TYPE org_status AS ENUM ('active', 'inactive');
CREATE TYPE cluster_status AS ENUM ('connected', 'pending', 'unreachable', 'auth_failed');
CREATE TYPE cluster_type AS ENUM ('pipeline', 'target');
CREATE TYPE member_role AS ENUM ('admin', 'devops', 'developer');
CREATE TYPE stack_status AS ENUM ('draft', 'deploying', 'deployed', 'failed', 'orphaned');
CREATE TYPE deployment_status AS ENUM (
    'pending', 'validating', 'installing', 'configuring',
    'healthcheck', 'completed', 'failed', 'rolling_back',
    'rolled_back', 'cancelled', 'timeout', 'retrying',
    'partial_success'
);
CREATE TYPE deployment_type AS ENUM ('stack', 'pipeline');
CREATE TYPE log_level AS ENUM ('debug', 'info', 'warn', 'error');
CREATE TYPE alert_channel AS ENUM ('slack', 'email');
CREATE TYPE alert_severity AS ENUM ('critical', 'warning', 'info');
CREATE TYPE pipeline_deploy_status AS ENUM ('pending', 'deploying', 'deployed', 'failed', 'rolled_back');
CREATE TYPE rbac_effect AS ENUM ('allow', 'deny');
CREATE TYPE compatibility_status AS ENUM ('verified', 'experimental', 'deprecated');
CREATE TYPE rollback_mode AS ENUM ('safe', 'destructive');
```

### 5.2 updated_at 자동 갱신 트리거

```sql
CREATE OR REPLACE FUNCTION trigger_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- 사용법: 각 테이블 생성 후 적용
-- CREATE TRIGGER set_updated_at
--     BEFORE UPDATE ON {table_name}
--     FOR EACH ROW
--     EXECUTE FUNCTION trigger_set_updated_at();
```

---

## 6. Context 1: Organization (조직 관리)

> 소유 모듈: `internal/admin/`
> Aggregate Root: Organization

### 6.1 organizations

```sql
CREATE TABLE organizations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(100) NOT NULL,
    slug            VARCHAR(50) NOT NULL,
    domain          VARCHAR(255),
    status          org_status NOT NULL DEFAULT 'active',
    default_admin_id UUID,                          -- users.id 느슨한 참조
    created_by      UUID,                           -- users.id 느슨한 참조
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ,

    CONSTRAINT uq_organizations_slug UNIQUE (slug)
);

CREATE TRIGGER set_updated_at
    BEFORE UPDATE ON organizations
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

-- 인덱스
CREATE INDEX idx_organizations_status ON organizations (status) WHERE deleted_at IS NULL;
CREATE INDEX idx_organizations_slug ON organizations (slug) WHERE deleted_at IS NULL;
```

**컬럼 설명:**

| 컬럼 | 타입 | 설명 |
|------|------|------|
| `id` | UUID | PK, 자동 생성 |
| `name` | VARCHAR(100) | 조직 표시명 |
| `slug` | VARCHAR(50) | URL 안전 식별자, 유니크 |
| `domain` | VARCHAR(255) | 회사 도메인 (선택) |
| `status` | org_status | 활성/비활성 |
| `default_admin_id` | UUID | 기본 관리자 (users.id, 느슨한 참조) |
| `created_by` | UUID | 생성자 (users.id, 느슨한 참조) |
| `created_at` | TIMESTAMPTZ | 생성 시각 |
| `updated_at` | TIMESTAMPTZ | 최종 수정 시각 |
| `deleted_at` | TIMESTAMPTZ | 소프트 삭제 시각 |

### 6.2 org_members

```sql
CREATE TABLE org_members (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL,                  -- users.id 느슨한 참조 (Auth Context)
    role            member_role NOT NULL DEFAULT 'developer',
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    invited_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    accepted_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_org_members_org_user UNIQUE (org_id, user_id)
);

CREATE TRIGGER set_updated_at
    BEFORE UPDATE ON org_members
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

-- 인덱스
CREATE INDEX idx_org_members_org_id ON org_members (org_id);
CREATE INDEX idx_org_members_user_id ON org_members (user_id);
CREATE INDEX idx_org_members_role ON org_members (org_id, role);
```

**컬럼 설명:**

| 컬럼 | 타입 | 설명 |
|------|------|------|
| `id` | UUID | PK |
| `org_id` | UUID | FK -> organizations.id |
| `user_id` | UUID | 사용자 ID (Auth Context 느슨한 참조) |
| `role` | member_role | admin / devops / developer (PRD v1.3 3역할 체계) |
| `is_active` | BOOLEAN | 활성 여부 (비활성화 시 접근 차단) |
| `invited_at` | TIMESTAMPTZ | 초대 시각 |
| `accepted_at` | TIMESTAMPTZ | 초대 수락 시각 (NULL이면 미수락) |

### 6.3 invite_links

```sql
CREATE TABLE invite_links (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    token           VARCHAR(255) NOT NULL,
    email           VARCHAR(255),
    role            member_role NOT NULL DEFAULT 'developer',
    expires_at      TIMESTAMPTZ NOT NULL,
    accepted_at     TIMESTAMPTZ,
    created_by      UUID,                           -- users.id 느슨한 참조
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_invite_links_token UNIQUE (token)
);

-- 인덱스
CREATE INDEX idx_invite_links_org_id ON invite_links (org_id);
CREATE INDEX idx_invite_links_token ON invite_links (token) WHERE accepted_at IS NULL;
CREATE INDEX idx_invite_links_expires ON invite_links (expires_at) WHERE accepted_at IS NULL;
```

**컬럼 설명:**

| 컬럼 | 타입 | 설명 |
|------|------|------|
| `token` | VARCHAR(255) | 초대 토큰 (URL에 포함) |
| `email` | VARCHAR(255) | 초대 대상 이메일 (선택) |
| `role` | member_role | 초대 시 부여할 역할 |
| `expires_at` | TIMESTAMPTZ | 만료 시각 (기본 7일) |
| `accepted_at` | TIMESTAMPTZ | 수락 시각 (NULL이면 미수락) |

### 6.4 org_cluster_access

```sql
CREATE TABLE org_cluster_access (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    cluster_id      UUID NOT NULL,                  -- clusters.id 느슨한 참조 (Cluster Context)
    access_type     VARCHAR(20) NOT NULL DEFAULT 'read_write',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_org_cluster_access UNIQUE (org_id, cluster_id)
);

-- 인덱스
CREATE INDEX idx_org_cluster_access_org ON org_cluster_access (org_id);
CREATE INDEX idx_org_cluster_access_cluster ON org_cluster_access (cluster_id);
```

**컬럼 설명:**

| 컬럼 | 타입 | 설명 |
|------|------|------|
| `org_id` | UUID | FK -> organizations.id |
| `cluster_id` | UUID | 클러스터 ID (Cluster Context 느슨한 참조) |
| `access_type` | VARCHAR(20) | 접근 유형 (read_only / read_write) |

---

## 7. Context 2: Cluster (클러스터 관리)

> 소유 모듈: `internal/admin/`
> Aggregate Root: Cluster

### 7.1 clusters

```sql
CREATE TABLE clusters (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id                UUID NOT NULL,              -- organizations.id 느슨한 참조
    name                  VARCHAR(100) NOT NULL,
    type                  cluster_type NOT NULL,
    endpoint              VARCHAR(500),
    namespace             VARCHAR(255),
    auth_method           VARCHAR(50) NOT NULL DEFAULT 'kubeconfig',
    kubeconfig_encrypted  BYTEA,                      -- AES-256-GCM 암호화된 kubeconfig
    encryption_iv         BYTEA,                      -- AES-256-GCM IV (12 bytes)
    encryption_tag        BYTEA,                      -- AES-256-GCM 인증 태그 (16 bytes)
    encryption_key_id     VARCHAR(50),                -- 암호화 키 식별자 (키 로테이션용)
    k8s_version           VARCHAR(20),
    node_architecture     VARCHAR(20),                -- amd64 / arm64
    status                cluster_status NOT NULL DEFAULT 'pending',
    last_verified_at      TIMESTAMPTZ,
    namespaces_cache      JSONB,                      -- 네임스페이스 목록 캐시
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at            TIMESTAMPTZ,

    CONSTRAINT uq_clusters_name_org UNIQUE (org_id, name)
);

CREATE TRIGGER set_updated_at
    BEFORE UPDATE ON clusters
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

-- 인덱스
CREATE INDEX idx_clusters_org_id ON clusters (org_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_clusters_status ON clusters (status) WHERE deleted_at IS NULL;
CREATE INDEX idx_clusters_type ON clusters (type) WHERE deleted_at IS NULL;
```

**컬럼 설명:**

| 컬럼 | 타입 | 설명 |
|------|------|------|
| `id` | UUID | PK |
| `org_id` | UUID | 소속 조직 (Org Context 느슨한 참조) |
| `name` | VARCHAR(100) | 클러스터 표시명 |
| `type` | cluster_type | pipeline (도구 설치용) / target (앱 배포용) |
| `endpoint` | VARCHAR(500) | K8s API Server 엔드포인트 |
| `namespace` | VARCHAR(255) | 기본 네임스페이스 |
| `auth_method` | VARCHAR(50) | 인증 방식 (kubeconfig / oidc / token) |
| `kubeconfig_encrypted` | BYTEA | AES-256-GCM으로 암호화된 kubeconfig |
| `encryption_iv` | BYTEA | AES-256-GCM 초기화 벡터 (12 bytes) |
| `encryption_tag` | BYTEA | AES-256-GCM 인증 태그 (16 bytes) |
| `encryption_key_id` | VARCHAR(50) | 암호화 키 ID (90일 주기 로테이션 추적) |
| `k8s_version` | VARCHAR(20) | 감지된 Kubernetes 버전 |
| `node_architecture` | VARCHAR(20) | 노드 아키텍처 (ARM64 대체 이미지 선택용) |
| `status` | cluster_status | 연결 상태 |
| `last_verified_at` | TIMESTAMPTZ | 마지막 연결 검증 시각 |
| `namespaces_cache` | JSONB | 네임스페이스 목록 캐시 (30초 동기화) |

---

## 8. Context 3: Stack (DevSecOps 스택 관리)

> 소유 모듈: `internal/stack/`
> Aggregate Root: Stack, GoldenPathTemplate, CompatibilityMatrix

### 8.1 golden_path_templates

```sql
CREATE TABLE golden_path_templates (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id         VARCHAR(100) NOT NULL,       -- 예: 'gitlab-allinone-v1'
    name                VARCHAR(200) NOT NULL,
    description         TEXT,
    tools_config        JSONB NOT NULL,               -- 도구 목록 및 버전
    resource_baseline   JSONB NOT NULL,               -- 기본 리소스 요구량
    estimated_install_minutes INT,                    -- 예상 설치 시간 (분)
    use_cases           TEXT[],                       -- 권장 사용 사례
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_golden_path_template_id UNIQUE (template_id)
);

CREATE TRIGGER set_updated_at
    BEFORE UPDATE ON golden_path_templates
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();
```

**컬럼 설명:**

| 컬럼 | 타입 | 설명 |
|------|------|------|
| `template_id` | VARCHAR(100) | 템플릿 식별자 (예: `gitlab-allinone-v1`) |
| `name` | VARCHAR(200) | 템플릿 표시명 (예: "GitLab All-in-One") |
| `tools_config` | JSONB | 도구 목록/버전 (JSONB 구조는 14절 참조) |
| `resource_baseline` | JSONB | 기본 리소스 요구량 (cpu_cores, memory_gi, storage_gi) |
| `estimated_install_minutes` | INT | 예상 설치 시간 |
| `use_cases` | TEXT[] | 권장 사용 사례 배열 |

**`tools_config` JSONB 구조:**
```json
{
  "source_repository": { "tool": "gitlab-ce", "helm_version": "8.7.x", "app_version": "17.7.x" },
  "ci_platform": { "tool": "gitlab-ci", "version": "17.7.x" },
  "cd_tool": { "tool": "argocd", "helm_version": "7.7.x", "app_version": "2.13.x" },
  "monitoring_collection": { "tool": "prometheus", "helm_version": "67.x", "app_version": "3.1.x" },
  "monitoring_visualization": { "tool": "grafana", "version": "11.4.x" },
  "storage_backend": { "tool": "minio", "helm_version": "5.3.x", "app_version": "2024.x" }
}
```

### 8.2 stack_configs

```sql
CREATE TABLE stack_configs (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id              UUID NOT NULL,                -- organizations.id 느슨한 참조
    cluster_id          UUID NOT NULL,                -- clusters.id 느슨한 참조
    name                VARCHAR(200) NOT NULL,
    golden_path_id      UUID,                         -- golden_path_templates.id (같은 Context, FK 가능)
    config_json         JSONB NOT NULL,               -- 스택 전체 설정 (JSONB 구조는 14절 참조)
    status              stack_status NOT NULL DEFAULT 'draft',
    current_version     INT NOT NULL DEFAULT 1,
    created_by          UUID,                         -- users.id 느슨한 참조
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at          TIMESTAMPTZ,

    CONSTRAINT fk_stack_golden_path FOREIGN KEY (golden_path_id)
        REFERENCES golden_path_templates(id) ON DELETE SET NULL
);

CREATE TRIGGER set_updated_at
    BEFORE UPDATE ON stack_configs
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

-- 인덱스
CREATE INDEX idx_stack_configs_org ON stack_configs (org_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_stack_configs_cluster ON stack_configs (cluster_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_stack_configs_status ON stack_configs (status) WHERE deleted_at IS NULL;
CREATE INDEX idx_stack_configs_config ON stack_configs USING GIN (config_json);
```

**컬럼 설명:**

| 컬럼 | 타입 | 설명 |
|------|------|------|
| `id` | UUID | PK |
| `org_id` | UUID | 소속 조직 (느슨한 참조) |
| `cluster_id` | UUID | 배포 대상 클러스터 (느슨한 참조) |
| `name` | VARCHAR(200) | 스택 이름 |
| `golden_path_id` | UUID | 사용한 Golden Path 템플릿 (FK) |
| `config_json` | JSONB | 5단계 워크플로우의 전체 설정 데이터 |
| `status` | stack_status | draft / deploying / deployed / failed / orphaned |
| `current_version` | INT | 현재 설정 버전 번호 |

### 8.3 stack_config_versions (이력 관리)

```sql
CREATE TABLE stack_config_versions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    stack_config_id     UUID NOT NULL REFERENCES stack_configs(id) ON DELETE CASCADE,
    version_number      INT NOT NULL,
    config_snapshot     JSONB NOT NULL,               -- 해당 버전의 전체 설정 스냅샷
    change_reason       TEXT,                         -- 변경 사유
    changed_by          UUID,                         -- users.id 느슨한 참조
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_stack_version UNIQUE (stack_config_id, version_number)
);

-- 인덱스
CREATE INDEX idx_stack_versions_config ON stack_config_versions (stack_config_id);
CREATE INDEX idx_stack_versions_number ON stack_config_versions (stack_config_id, version_number DESC);
```

**컬럼 설명:**

| 컬럼 | 타입 | 설명 |
|------|------|------|
| `stack_config_id` | UUID | FK -> stack_configs.id |
| `version_number` | INT | 버전 번호 (1부터 자동 증가) |
| `config_snapshot` | JSONB | 해당 버전 시점의 전체 설정 스냅샷 |
| `change_reason` | TEXT | 변경 사유 (사용자 입력) |
| `changed_by` | UUID | 변경자 (users.id 느슨한 참조) |

### 8.4 stack_helm_step_configs

```sql
CREATE TABLE stack_helm_step_configs (
    step_name    VARCHAR(100) PRIMARY KEY,
    release_name VARCHAR(255),
    chart_name   VARCHAR(255) NOT NULL,
    repo_url     VARCHAR(512),
    version      VARCHAR(100),
    namespace    VARCHAR(255),
    phase        VARCHAR(10) NOT NULL,
    sort_order   SMALLINT NOT NULL,
    wait         BOOLEAN NOT NULL DEFAULT false,
    is_enabled   BOOLEAN NOT NULL DEFAULT true,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_stack_helm_step_configs_order ON stack_helm_step_configs (sort_order, step_name);
```

**컬럼 설명:**

| 컬럼 | 타입 | 설명 |
|------|------|------|
| `step_name` | VARCHAR(100) | 설치 step 식별자 |
| `release_name` | VARCHAR(255) | Helm release 이름 (없으면 chart_name 사용) |
| `chart_name` | VARCHAR(255) | Helm chart 이름 또는 OCI ref |
| `repo_url` | VARCHAR(512) | Helm repo URL |
| `version` | VARCHAR(100) | 설치 chart 버전 |
| `namespace` | VARCHAR(255) | 설치 대상 네임스페이스 |
| `phase` | VARCHAR(10) | 설치 phase (A/B/C) |
| `sort_order` | SMALLINT | 실행 순서 |
| `wait` | BOOLEAN | Helm wait 옵션 |
| `is_enabled` | BOOLEAN | 관리상 활성화 여부 |

### 8.5 deployments

```sql
CREATE TABLE deployments (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    stack_config_id     UUID NOT NULL REFERENCES stack_configs(id) ON DELETE CASCADE,
    type                deployment_type NOT NULL DEFAULT 'stack',
    version_number      INT,                          -- 배포 시점의 stack version
    status              deployment_status NOT NULL DEFAULT 'pending',
    rollback_mode       rollback_mode NOT NULL DEFAULT 'safe',
    started_by          UUID,                         -- users.id 느슨한 참조
    started_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at        TIMESTAMPTZ,
    error_message       TEXT,
    helm_releases       JSONB,                        -- 설치된 Helm 릴리스 목록
    rollback_stack      JSONB,                        -- 롤백 시 실행할 작업 스택 (역순)
    install_phase       VARCHAR(20),                  -- 현재 Phase (A/B/C)
    progress_percent    SMALLINT DEFAULT 0,           -- 진행률 (0-100)
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TRIGGER set_updated_at
    BEFORE UPDATE ON deployments
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

-- 인덱스
CREATE INDEX idx_deployments_stack ON deployments (stack_config_id);
CREATE INDEX idx_deployments_status ON deployments (status);
CREATE INDEX idx_deployments_started ON deployments (started_at DESC);
CREATE INDEX idx_deployments_type ON deployments (type);
```

**컬럼 설명:**

| 컬럼 | 타입 | 설명 |
|------|------|------|
| `id` | UUID | PK |
| `stack_config_id` | UUID | FK -> stack_configs.id |
| `type` | deployment_type | stack (스택 배포) / pipeline (파이프라인 배포) |
| `version_number` | INT | 배포 시점 스택 설정 버전 |
| `status` | deployment_status | 상태 머신 (PRD 참조: PENDING -> VALIDATING -> INSTALLING -> ...) |
| `rollback_mode` | rollback_mode | safe (PVC 보존) / destructive (전체 삭제) |
| `helm_releases` | JSONB | 설치된 Helm 릴리스 목록 |
| `rollback_stack` | JSONB | 롤백 시 실행할 작업 스택 (역순 uninstall) |
| `install_phase` | VARCHAR(20) | 3-Phase 프로비저닝 현재 단계 (A/B/C) |
| `progress_percent` | SMALLINT | 진행률 (WebSocket으로 클라이언트에 전송) |

**`helm_releases` JSONB 구조:**
```json
[
  { "name": "minio", "namespace": "nullus-artifacts", "chart": "minio/minio", "version": "5.3.0", "status": "deployed" },
  { "name": "gitlab", "namespace": "nullus-scm", "chart": "gitlab/gitlab", "version": "8.7.0", "status": "deployed" },
  { "name": "argocd", "namespace": "nullus-cicd", "chart": "argo/argo-cd", "version": "7.7.0", "status": "deployed" }
]
```

### 8.6 deployment_steps

```sql
CREATE TABLE deployment_steps (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deployment_id       UUID NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
    step_order          SMALLINT NOT NULL,
    step_name           VARCHAR(100) NOT NULL,        -- 예: 'install_minio', 'install_gitlab'
    phase               VARCHAR(10) NOT NULL,         -- A / B / C
    status              deployment_status NOT NULL DEFAULT 'pending',
    helm_chart          VARCHAR(200),
    helm_version        VARCHAR(50),
    namespace           VARCHAR(255),
    started_at          TIMESTAMPTZ,
    completed_at        TIMESTAMPTZ,
    error_message       TEXT,
    retry_count         SMALLINT NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_deployment_step UNIQUE (deployment_id, step_order)
);

-- 인덱스
CREATE INDEX idx_deployment_steps_deployment ON deployment_steps (deployment_id);
CREATE INDEX idx_deployment_steps_status ON deployment_steps (deployment_id, status);
```

**컬럼 설명:**

| 컬럼 | 타입 | 설명 |
|------|------|------|
| `step_order` | SMALLINT | 실행 순서 |
| `step_name` | VARCHAR(100) | 단계명 (예: install_minio, install_gitlab) |
| `phase` | VARCHAR(10) | 3-Phase 소속 (A: 인프라, B: 플랫폼, C: 연동) |
| `helm_chart` | VARCHAR(200) | Helm 차트 (예: minio/minio) |
| `namespace` | VARCHAR(255) | 설치 대상 네임스페이스 |
| `retry_count` | SMALLINT | 재시도 횟수 |

### 8.7 deployment_logs

```sql
CREATE TABLE deployment_logs (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deployment_id       UUID NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
    step_name           VARCHAR(100),
    level               log_level NOT NULL DEFAULT 'info',
    message             TEXT NOT NULL,
    metadata            JSONB,                        -- 추가 컨텍스트 (에러 스택 등)
    timestamp           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 인덱스 (시계열 조회 최적화)
CREATE INDEX idx_deployment_logs_deployment ON deployment_logs (deployment_id, timestamp);
CREATE INDEX idx_deployment_logs_level ON deployment_logs (deployment_id, level);

-- 파티셔닝 고려: 로그량이 많아지면 월별 파티셔닝 적용
-- CREATE TABLE deployment_logs (...) PARTITION BY RANGE (timestamp);
```

**컬럼 설명:**

| 컬럼 | 타입 | 설명 |
|------|------|------|
| `deployment_id` | UUID | FK -> deployments.id |
| `step_name` | VARCHAR(100) | 관련 설치 단계 |
| `level` | log_level | debug / info / warn / error |
| `message` | TEXT | 로그 메시지 (WebSocket으로 스트리밍) |
| `metadata` | JSONB | 추가 컨텍스트 (에러 스택트레이스 등) |
| `timestamp` | TIMESTAMPTZ | 로그 발생 시각 |

### 8.8 compatibility_matrices

```sql
CREATE TABLE compatibility_matrices (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    matrix_id           VARCHAR(100) NOT NULL,        -- 예: 'gitlab-allinone-v1'
    name                VARCHAR(200) NOT NULL,
    status              compatibility_status NOT NULL DEFAULT 'experimental',
    k8s_min_version     VARCHAR(20) NOT NULL,
    k8s_max_version     VARCHAR(20),
    k8s_recommended     VARCHAR(20),
    tools               JSONB NOT NULL,               -- 도구별 helm_version / app_version
    integration_tests   TEXT[],                       -- 통합 테스트 목록
    tested_at           TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_compatibility_matrix_id UNIQUE (matrix_id)
);

CREATE TRIGGER set_updated_at
    BEFORE UPDATE ON compatibility_matrices
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

-- 인덱스
CREATE INDEX idx_compatibility_status ON compatibility_matrices (status);
CREATE INDEX idx_compatibility_tools ON compatibility_matrices USING GIN (tools);
```

**컬럼 설명:**

| 컬럼 | 타입 | 설명 |
|------|------|------|
| `matrix_id` | VARCHAR(100) | 매트릭스 식별자 |
| `status` | compatibility_status | verified / experimental / deprecated |
| `k8s_min_version` | VARCHAR(20) | 최소 Kubernetes 버전 (1.26) |
| `k8s_max_version` | VARCHAR(20) | 최대 Kubernetes 버전 (1.30) |
| `k8s_recommended` | VARCHAR(20) | 권장 Kubernetes 버전 (1.28) |
| `tools` | JSONB | 도구별 chart/app 버전 분리 관리 (Narwhal VERSIONS.md 패턴) |
| `integration_tests` | TEXT[] | 검증된 통합 테스트 항목 |

**`tools` JSONB 구조 (Chart/App 버전 분리):**
```json
{
  "source_repository": {
    "name": "gitlab-ce",
    "helm_chart": "gitlab/gitlab",
    "helm_version": "8.7.x",
    "app_version": "17.7.x"
  },
  "cd_tool": {
    "name": "argocd",
    "helm_chart": "argo/argo-cd",
    "helm_version": "7.7.x",
    "app_version": "2.13.x"
  },
  "monitoring_collection": {
    "name": "prometheus",
    "helm_chart": "prometheus-community/kube-prometheus-stack",
    "helm_version": "67.x",
    "app_version": "3.1.x"
  }
}
```

---

## 9. Context 4: CI/CD (파이프라인 관리)

> 소유 모듈: `internal/cicd/`
> Aggregate Root: Pipeline

### 9.1 pipeline_templates

```sql
CREATE TABLE pipeline_templates (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id         VARCHAR(100) NOT NULL,        -- 예: 'web-backend-v1'
    name                VARCHAR(200) NOT NULL,
    description         TEXT,
    category            VARCHAR(50) NOT NULL,         -- frontend / backend / fullstack / batch
    stages              JSONB NOT NULL,               -- 파이프라인 단계 정의
    variables           JSONB,                        -- 템플릿 변수 정의
    default_config      JSONB,                        -- 기본 CI 설정 (예: .gitlab-ci.yml)
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    current_version     INT NOT NULL DEFAULT 1,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_pipeline_template_id UNIQUE (template_id)
);

CREATE TRIGGER set_updated_at
    BEFORE UPDATE ON pipeline_templates
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

-- 인덱스
CREATE INDEX idx_pipeline_templates_category ON pipeline_templates (category) WHERE is_active = TRUE;
```

**컬럼 설명:**

| 컬럼 | 타입 | 설명 |
|------|------|------|
| `template_id` | VARCHAR(100) | 템플릿 식별자 (예: web-backend-v1) |
| `category` | VARCHAR(50) | 카테고리 (frontend / backend / fullstack / batch) |
| `stages` | JSONB | 파이프라인 단계 정의 (Build -> Test -> Image Build -> Deploy) |
| `variables` | JSONB | 사용자 입력 변수 정의 (repo_url, image_name 등) |
| `default_config` | JSONB | 기본 CI 설정 파일 내용 |

### 9.2 pipeline_template_versions

```sql
CREATE TABLE pipeline_template_versions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id         UUID NOT NULL REFERENCES pipeline_templates(id) ON DELETE CASCADE,
    version_number      INT NOT NULL,
    stages_snapshot     JSONB NOT NULL,
    change_note         TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_pipeline_tmpl_version UNIQUE (template_id, version_number)
);

-- 인덱스
CREATE INDEX idx_pipeline_tmpl_versions ON pipeline_template_versions (template_id);
```

### 9.3 pipeline_configs

```sql
CREATE TABLE pipeline_configs (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id              UUID NOT NULL,                -- organizations.id 느슨한 참조
    stack_config_id     UUID,                         -- stack_configs.id 느슨한 참조
    template_id         UUID REFERENCES pipeline_templates(id) ON DELETE SET NULL,
    name                VARCHAR(200) NOT NULL,
    app_name            VARCHAR(100),
    repo_url            VARCHAR(500),
    cluster_id          UUID,                         -- clusters.id 느슨한 참조
    namespace           VARCHAR(255),
    params_json         JSONB NOT NULL DEFAULT '{}',  -- 파라미터 (이미지명, 환경변수 등)
    ci_config           TEXT,                         -- CI 설정 파일 내용 (.gitlab-ci.yml 등)
    status              VARCHAR(20) NOT NULL DEFAULT 'created',
    created_by          UUID,                         -- users.id 느슨한 참조
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at          TIMESTAMPTZ
);

CREATE TRIGGER set_updated_at
    BEFORE UPDATE ON pipeline_configs
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

-- 인덱스
CREATE INDEX idx_pipeline_configs_org ON pipeline_configs (org_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_pipeline_configs_stack ON pipeline_configs (stack_config_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_pipeline_configs_status ON pipeline_configs (status) WHERE deleted_at IS NULL;
CREATE INDEX idx_pipeline_configs_params ON pipeline_configs USING GIN (params_json);
```

**컬럼 설명:**

| 컬럼 | 타입 | 설명 |
|------|------|------|
| `stack_config_id` | UUID | 연결된 스택 (느슨한 참조) |
| `template_id` | UUID | FK -> pipeline_templates.id |
| `app_name` | VARCHAR(100) | 애플리케이션 이름 |
| `repo_url` | VARCHAR(500) | Git 저장소 URL |
| `cluster_id` | UUID | 배포 대상 클러스터 (느슨한 참조) |
| `namespace` | VARCHAR(255) | 배포 대상 네임스페이스 |
| `params_json` | JSONB | 파라미터 (이미지명, 환경변수, 리소스 설정 등) |
| `ci_config` | TEXT | CI 설정 파일 내용 |

### 9.4 pipeline_deployments

```sql
CREATE TABLE pipeline_deployments (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pipeline_config_id  UUID NOT NULL REFERENCES pipeline_configs(id) ON DELETE CASCADE,
    version             INT NOT NULL,
    status              pipeline_deploy_status NOT NULL DEFAULT 'pending',
    k8s_objects         JSONB,                        -- 생성된 K8s 오브젝트 목록
    config_snapshot     JSONB,                        -- 배포 시점 파이프라인 설정 스냅샷
    deployed_by         UUID,                         -- users.id 느슨한 참조
    deployed_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at        TIMESTAMPTZ,
    error_message       TEXT,
    change_reason       TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_pipeline_deployment_version UNIQUE (pipeline_config_id, version)
);

-- 인덱스
CREATE INDEX idx_pipeline_deployments_config ON pipeline_deployments (pipeline_config_id);
CREATE INDEX idx_pipeline_deployments_status ON pipeline_deployments (status);
CREATE INDEX idx_pipeline_deployments_deployed ON pipeline_deployments (deployed_at DESC);
```

**컬럼 설명:**

| 컬럼 | 타입 | 설명 |
|------|------|------|
| `pipeline_config_id` | UUID | FK -> pipeline_configs.id |
| `version` | INT | 배포 버전 (1부터 자동 증가) |
| `k8s_objects` | JSONB | 생성된 K8s 오브젝트 (Namespace, Deployment, Service, Ingress 등) |
| `config_snapshot` | JSONB | 배포 시점의 파이프라인 설정 스냅샷 (diff용) |
| `change_reason` | TEXT | 변경 사유 |

**`k8s_objects` JSONB 구조:**
```json
{
  "namespace": { "name": "app-my-service", "status": "created" },
  "deployments": [{ "name": "my-service", "replicas": 2 }],
  "services": [{ "name": "my-service", "type": "ClusterIP", "port": 8080 }],
  "ingresses": [{ "name": "my-service", "host": "my-service.example.com" }],
  "secrets": ["my-service-env"],
  "pvcs": [{ "name": "my-service-data", "size": "10Gi" }]
}
```

---

## 10. Context 5: Observability (관측성)

> 소유 모듈: `internal/observability/`
> Aggregate Root: Alert

### 10.1 alert_configs

```sql
CREATE TABLE alert_configs (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id              UUID NOT NULL,                -- organizations.id 느슨한 참조
    stack_config_id     UUID,                         -- stack_configs.id 느슨한 참조
    name                VARCHAR(200) NOT NULL,
    channel             alert_channel NOT NULL,
    webhook_url_encrypted BYTEA,                     -- Slack Webhook URL (AES-256-GCM 암호화)
    smtp_config         JSONB,                        -- Email SMTP 설정
    event_types         TEXT[] NOT NULL DEFAULT ARRAY[
        'tool_down', 'high_cpu', 'high_memory',
        'storage_warning', 'pipeline_failure'
    ],
    thresholds          JSONB,                        -- 알림 임계값 설정
    enabled             BOOLEAN NOT NULL DEFAULT TRUE,
    created_by          UUID,                         -- users.id 느슨한 참조
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TRIGGER set_updated_at
    BEFORE UPDATE ON alert_configs
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

-- 인덱스
CREATE INDEX idx_alert_configs_org ON alert_configs (org_id);
CREATE INDEX idx_alert_configs_enabled ON alert_configs (enabled) WHERE enabled = TRUE;
```

**컬럼 설명:**

| 컬럼 | 타입 | 설명 |
|------|------|------|
| `channel` | alert_channel | slack / email |
| `webhook_url_encrypted` | BYTEA | Slack Webhook URL (암호화 저장) |
| `smtp_config` | JSONB | Email SMTP 설정 (host, port, from, to) |
| `event_types` | TEXT[] | 감시 이벤트 유형 배열 |
| `thresholds` | JSONB | 알림 임계값 (cpu_percent, memory_percent 등) |

### 10.2 alert_history

```sql
CREATE TABLE alert_history (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    alert_config_id     UUID REFERENCES alert_configs(id) ON DELETE SET NULL,
    org_id              UUID NOT NULL,                -- organizations.id 느슨한 참조
    event_type          VARCHAR(50) NOT NULL,
    severity            alert_severity NOT NULL,
    title               VARCHAR(300) NOT NULL,
    message             TEXT NOT NULL,
    metadata            JSONB,                        -- 추가 컨텍스트 (메트릭 값 등)
    acknowledged        BOOLEAN NOT NULL DEFAULT FALSE,
    acknowledged_by     UUID,                         -- users.id 느슨한 참조
    acknowledged_at     TIMESTAMPTZ,
    fired_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at         TIMESTAMPTZ
);

-- 인덱스 (시계열 조회 최적화)
CREATE INDEX idx_alert_history_org ON alert_history (org_id, fired_at DESC);
CREATE INDEX idx_alert_history_severity ON alert_history (severity, fired_at DESC);
CREATE INDEX idx_alert_history_event ON alert_history (event_type);
CREATE INDEX idx_alert_history_unack ON alert_history (org_id)
    WHERE acknowledged = FALSE AND resolved_at IS NULL;
```

**컬럼 설명:**

| 컬럼 | 타입 | 설명 |
|------|------|------|
| `alert_config_id` | UUID | FK -> alert_configs.id (삭제 시 NULL) |
| `event_type` | VARCHAR(50) | 이벤트 유형 (tool_down, high_cpu 등) |
| `severity` | alert_severity | critical / warning / info |
| `metadata` | JSONB | 추가 컨텍스트 (메트릭 값, Pod 이름 등) |
| `acknowledged` | BOOLEAN | 확인 여부 |
| `fired_at` | TIMESTAMPTZ | 알림 발생 시각 |
| `resolved_at` | TIMESTAMPTZ | 해소 시각 (NULL이면 미해소) |

---

## 11. Context 6: Auth (인증/인가)

> 소유 모듈: `internal/auth/`
> Aggregate Root: User, Session

### 11.1 users

```sql
CREATE TABLE users (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email               VARCHAR(255) NOT NULL,
    password_hash       VARCHAR(255),                 -- bcrypt 해시 (OIDC 사용자는 NULL)
    display_name        VARCHAR(100),
    avatar_url          VARCHAR(500),
    auth_provider       VARCHAR(50) NOT NULL DEFAULT 'local',  -- local / keycloak
    external_id         VARCHAR(255),                 -- Keycloak subject ID
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    last_login_at       TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at          TIMESTAMPTZ,

    CONSTRAINT uq_users_email UNIQUE (email)
);

CREATE TRIGGER set_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

-- 인덱스
CREATE INDEX idx_users_email ON users (email) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_provider ON users (auth_provider) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_external ON users (external_id) WHERE external_id IS NOT NULL;
```

**컬럼 설명:**

| 컬럼 | 타입 | 설명 |
|------|------|------|
| `email` | VARCHAR(255) | 로그인 이메일 (UNIQUE) |
| `password_hash` | VARCHAR(255) | bcrypt 해시 (Alpha/Beta: 세션 인증, v1: OIDC 시 NULL) |
| `auth_provider` | VARCHAR(50) | 인증 제공자 (local / keycloak) |
| `external_id` | VARCHAR(255) | Keycloak subject ID (v1 OIDC 연동) |
| `is_active` | BOOLEAN | 활성 여부 (비활성 시 로그인 차단) |

### 11.2 sessions

```sql
CREATE TABLE sessions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash          VARCHAR(255) NOT NULL,        -- 세션 토큰 해시
    ip_address          INET,
    user_agent          TEXT,
    expires_at          TIMESTAMPTZ NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_sessions_token UNIQUE (token_hash)
);

-- 인덱스
CREATE INDEX idx_sessions_user ON sessions (user_id);
CREATE INDEX idx_sessions_expires ON sessions (expires_at);
```

**컬럼 설명:**

| 컬럼 | 타입 | 설명 |
|------|------|------|
| `token_hash` | VARCHAR(255) | 세션 토큰의 SHA-256 해시 |
| `ip_address` | INET | 로그인 IP |
| `user_agent` | TEXT | 클라이언트 User-Agent |
| `expires_at` | TIMESTAMPTZ | 세션 만료 시각 (액세스 15분, 리프레시 7일) |

### 11.3 rbac_policies

```sql
CREATE TABLE rbac_policies (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    role_name           member_role NOT NULL,
    resource_type       VARCHAR(50) NOT NULL,         -- organization, cluster, stack, pipeline, monitoring, alert, user
    action              VARCHAR(50) NOT NULL,         -- create, read, update, delete, deploy, rollback
    effect              rbac_effect NOT NULL DEFAULT 'allow',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_rbac_policy UNIQUE (role_name, resource_type, action)
);

-- 인덱스
CREATE INDEX idx_rbac_role ON rbac_policies (role_name);
```

**컬럼 설명:**

| 컬럼 | 타입 | 설명 |
|------|------|------|
| `role_name` | member_role | admin / devops / developer |
| `resource_type` | VARCHAR(50) | 리소스 유형 |
| `action` | VARCHAR(50) | 허용 액션 |
| `effect` | rbac_effect | allow / deny |

**초기 RBAC 데이터 (PRD v1.3 3역할 체계):**

```sql
-- Admin: 전체 권한
INSERT INTO rbac_policies (role_name, resource_type, action, effect) VALUES
    ('admin', 'organization', 'create', 'allow'),
    ('admin', 'organization', 'read', 'allow'),
    ('admin', 'organization', 'update', 'allow'),
    ('admin', 'organization', 'delete', 'allow'),
    ('admin', 'cluster', 'create', 'allow'),
    ('admin', 'cluster', 'read', 'allow'),
    ('admin', 'cluster', 'update', 'allow'),
    ('admin', 'cluster', 'delete', 'allow'),
    ('admin', 'stack', 'create', 'allow'),
    ('admin', 'stack', 'read', 'allow'),
    ('admin', 'stack', 'update', 'allow'),
    ('admin', 'stack', 'deploy', 'allow'),
    ('admin', 'pipeline', 'create', 'allow'),
    ('admin', 'pipeline', 'read', 'allow'),
    ('admin', 'pipeline', 'deploy', 'allow'),
    ('admin', 'pipeline', 'rollback', 'allow'),
    ('admin', 'monitoring', 'read', 'allow'),
    ('admin', 'alert', 'create', 'allow'),
    ('admin', 'alert', 'read', 'allow'),
    ('admin', 'user', 'create', 'allow'),
    ('admin', 'user', 'read', 'allow'),
    ('admin', 'user', 'update', 'allow'),
    ('admin', 'user', 'delete', 'allow');

-- DevOps Engineer: 스택/파이프라인 관리 권한
INSERT INTO rbac_policies (role_name, resource_type, action, effect) VALUES
    ('devops', 'cluster', 'read', 'allow'),
    ('devops', 'stack', 'create', 'allow'),
    ('devops', 'stack', 'read', 'allow'),
    ('devops', 'stack', 'update', 'allow'),
    ('devops', 'stack', 'deploy', 'allow'),
    ('devops', 'pipeline', 'create', 'allow'),
    ('devops', 'pipeline', 'read', 'allow'),
    ('devops', 'pipeline', 'deploy', 'allow'),
    ('devops', 'pipeline', 'rollback', 'allow'),
    ('devops', 'monitoring', 'read', 'allow'),
    ('devops', 'alert', 'create', 'allow'),
    ('devops', 'alert', 'read', 'allow');

-- Developer: 읽기 + 파이프라인 배포 권한
INSERT INTO rbac_policies (role_name, resource_type, action, effect) VALUES
    ('developer', 'cluster', 'read', 'allow'),
    ('developer', 'stack', 'read', 'allow'),
    ('developer', 'pipeline', 'read', 'allow'),
    ('developer', 'pipeline', 'deploy', 'allow'),
    ('developer', 'monitoring', 'read', 'allow'),
    ('developer', 'alert', 'read', 'allow');
```

### 11.4 menu_permissions

```sql
CREATE TABLE menu_permissions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    menu_id             VARCHAR(100) NOT NULL,        -- 메뉴 식별자
    menu_label_ko       VARCHAR(100),                 -- 한글 메뉴명
    menu_label_en       VARCHAR(100),                 -- 영문 메뉴명
    allowed_roles       member_role[] NOT NULL,       -- 접근 가능 역할 배열
    sort_order          SMALLINT NOT NULL DEFAULT 0,
    parent_menu_id      VARCHAR(100),                 -- 상위 메뉴 ID
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_menu_permissions_id UNIQUE (menu_id)
);

-- 인덱스
CREATE INDEX idx_menu_permissions_parent ON menu_permissions (parent_menu_id);
```

**초기 메뉴 데이터 (PRD v1.3 proto4 기준):**

```sql
INSERT INTO menu_permissions (menu_id, menu_label_ko, menu_label_en, allowed_roles, sort_order, parent_menu_id) VALUES
    -- 데브섹옵스 스택
    ('dss', '데브섹옵스 스택', 'DevSecOps Stack', ARRAY['admin','devops']::member_role[], 10, NULL),
    ('dss.template', '템플릿', 'Template', ARRAY['admin','devops']::member_role[], 11, 'dss'),
    ('dss.install', '설치', 'Install', ARRAY['admin','devops']::member_role[], 12, 'dss'),
    ('dss.list', '목록', 'List', ARRAY['admin','devops']::member_role[], 13, 'dss'),
    ('dss.history', '이력', 'History', ARRAY['admin','devops']::member_role[], 14, 'dss'),
    ('dss.version', '버전 관리', 'Version Management', ARRAY['admin','devops']::member_role[], 15, 'dss'),
    -- CI/CD
    ('cicd', 'CI/CD', 'CI/CD', ARRAY['admin','devops','developer']::member_role[], 20, NULL),
    ('cicd.template', '템플릿', 'Template', ARRAY['admin','devops','developer']::member_role[], 21, 'cicd'),
    ('cicd.list', '목록', 'List', ARRAY['admin','devops','developer']::member_role[], 22, 'cicd'),
    ('cicd.history', '이력', 'History', ARRAY['admin','devops','developer']::member_role[], 23, 'cicd'),
    -- 관측성
    ('obs', '관측성', 'Observability', ARRAY['admin','devops','developer']::member_role[], 30, NULL),
    ('obs.dashboard', '모니터링 대시보드', 'Monitoring Dashboard', ARRAY['admin','devops','developer']::member_role[], 31, 'obs'),
    ('obs.alert_rules', '알림 규칙', 'Alert Rules', ARRAY['admin','devops']::member_role[], 32, 'obs'),
    ('obs.alert_history', '알림 이력', 'Alert History', ARRAY['admin','devops','developer']::member_role[], 33, 'obs'),
    -- 관리
    ('admin', '관리', 'Admin', ARRAY['admin']::member_role[], 40, NULL),
    ('admin.org', '조직 관리', 'Organization', ARRAY['admin']::member_role[], 41, 'admin'),
    ('admin.users', '사용자 관리', 'User Management', ARRAY['admin']::member_role[], 42, 'admin'),
    ('admin.clusters', '클러스터 관리', 'Cluster Management', ARRAY['admin','devops']::member_role[], 43, 'admin'),
    -- 사용자
    ('user', '사용자', 'User', ARRAY['admin','devops','developer']::member_role[], 50, NULL),
    ('user.logout', '로그아웃', 'Log out', ARRAY['admin','devops','developer']::member_role[], 51, 'user');
```

---

## 12. 공통 테이블: 감사 로그

> 소유 모듈: `internal/shared/`
> 모든 Context에서 기록, 독립 테이블로 관리

### 12.1 audit_logs

```sql
CREATE TABLE audit_logs (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id            UUID,                         -- 수행자 (users.id, 시스템이면 NULL)
    actor_email         VARCHAR(255),                 -- 수행자 이메일 (비정규화, 조회 편의)
    actor_role          member_role,                  -- 수행 시점의 역할
    org_id              UUID,                         -- 소속 조직
    action              VARCHAR(100) NOT NULL,        -- 수행 액션
    resource_type       VARCHAR(50) NOT NULL,         -- 대상 리소스 유형
    resource_id         UUID,                         -- 대상 리소스 ID
    resource_name       VARCHAR(200),                 -- 대상 리소스 이름 (비정규화)
    changes             JSONB,                        -- 변경 전/후 값
    request_metadata    JSONB,                        -- IP, User-Agent, 요청 경로 등
    result              VARCHAR(20) NOT NULL DEFAULT 'success',  -- success / failure
    error_message       TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 인덱스 (시계열 + 필터 최적화)
CREATE INDEX idx_audit_logs_actor ON audit_logs (actor_id, created_at DESC);
CREATE INDEX idx_audit_logs_org ON audit_logs (org_id, created_at DESC);
CREATE INDEX idx_audit_logs_resource ON audit_logs (resource_type, resource_id);
CREATE INDEX idx_audit_logs_action ON audit_logs (action, created_at DESC);
CREATE INDEX idx_audit_logs_created ON audit_logs (created_at DESC);

-- 파티셔닝: 보존 기간 1년, 월별 파티셔닝 권장
-- CREATE TABLE audit_logs (...) PARTITION BY RANGE (created_at);
```

**컬럼 설명:**

| 컬럼 | 타입 | 설명 |
|------|------|------|
| `actor_id` | UUID | 수행자 ID (시스템 자동 작업은 NULL) |
| `actor_email` | VARCHAR(255) | 수행자 이메일 (비정규화, 빠른 조회용) |
| `actor_role` | member_role | 수행 시점의 역할 (이후 역할 변경과 무관하게 기록) |
| `action` | VARCHAR(100) | 수행 액션 (예: organization.create, cluster.delete, stack.deploy) |
| `resource_type` | VARCHAR(50) | 대상 리소스 유형 (organization, cluster, stack, pipeline 등) |
| `resource_id` | UUID | 대상 리소스 ID |
| `changes` | JSONB | 변경 전/후 값 (diff 형태) |
| `request_metadata` | JSONB | 요청 메타데이터 (IP, User-Agent, 경로) |
| `result` | VARCHAR(20) | 결과 (success / failure) |

**`changes` JSONB 구조:**
```json
{
  "before": { "status": "active", "name": "old-name" },
  "after": { "status": "inactive", "name": "new-name" }
}
```

**`request_metadata` JSONB 구조:**
```json
{
  "ip": "192.168.1.100",
  "user_agent": "Mozilla/5.0 ...",
  "method": "PUT",
  "path": "/api/v1/orgs/abc-123",
  "trace_id": "trace-xyz-456"
}
```

**감사 로그 기록 대상 (보안 운영 정책: 관리자 작업 전건 기록):**

| 액션 | 설명 |
|------|------|
| `organization.create` | 조직 생성 |
| `organization.update` | 조직 수정 |
| `organization.delete` | 조직 삭제 |
| `organization.status_change` | 조직 활성/비활성 전환 |
| `member.invite` | 멤버 초대 |
| `member.role_change` | 멤버 역할 변경 |
| `member.deactivate` | 멤버 비활성화 |
| `member.remove` | 멤버 제거 |
| `cluster.create` | 클러스터 등록 |
| `cluster.update` | 클러스터 수정 |
| `cluster.delete` | 클러스터 삭제 |
| `cluster.verify` | 클러스터 연결 검증 |
| `stack.create` | 스택 설정 생성 |
| `stack.update` | 스택 설정 수정 |
| `stack.deploy` | 스택 배포 시작 |
| `stack.rollback` | 스택 롤백 |
| `stack.delete` | 스택 삭제 |
| `pipeline.create` | 파이프라인 생성 |
| `pipeline.deploy` | 파이프라인 배포 |
| `pipeline.rollback` | 파이프라인 롤백 |
| `user.login` | 로그인 |
| `user.logout` | 로그아웃 |
| `user.role_change` | 사용자 역할 변경 |
| `user.deactivate` | 사용자 비활성화 |

---

## 13. Kubeconfig 암호화 저장 방식

### 13.1 암호화 알고리즘: AES-256-GCM

PRD 요구사항에 따라 Kubeconfig는 서버 측 DB에 AES-256-GCM으로 암호화 저장한다.

```
┌─ Kubeconfig 암호화 흐름 ────────────────────────────────────┐
│                                                               │
│  1. 사용자가 kubeconfig 파일 업로드                            │
│     ↓                                                         │
│  2. API Server에서 YAML 파싱 및 유효성 검증                    │
│     ↓                                                         │
│  3. AES-256-GCM 암호화                                        │
│     - 키: 환경변수 NULLUS_ENCRYPTION_KEY (32 bytes)           │
│     - IV: crypto/rand로 생성 (12 bytes, 매 암호화마다 새로 생성) │
│     - 출력: ciphertext + authentication tag (16 bytes)         │
│     ↓                                                         │
│  4. DB 저장                                                    │
│     - kubeconfig_encrypted: ciphertext (BYTEA)                │
│     - encryption_iv: IV (BYTEA, 12 bytes)                     │
│     - encryption_tag: authentication tag (BYTEA, 16 bytes)    │
│     - encryption_key_id: 키 식별자 (키 로테이션 추적)          │
│                                                               │
│  5. 사용 시 복호화 (메모리에서만)                               │
│     - DB에서 ciphertext + IV + tag 조회                       │
│     - encryption_key_id로 올바른 키 선택                      │
│     - AES-256-GCM 복호화 → 평문 kubeconfig                   │
│     - kubectl/client-go에 전달                                │
│     - 사용 완료 후 메모리에서 즉시 제거 (zeroing)              │
│                                                               │
└───────────────────────────────────────────────────────────────┘
```

### 13.2 키 관리

| 항목 | 정책 |
|------|------|
| **암호화 키 소스** | **OpenBao KV 경로(기본)** + 로컬 개발용 `NULLUS_ENCRYPTION_KEY` fallback |
| **키 로테이션 주기** | 90일 (보안 운영 정책) |
| **키 로테이션 방식** | 새 키 ID 발급 -> 새 암호화에 새 키 사용 -> 기존 데이터는 배치 작업으로 재암호화 |
| **키 백업** | OpenBao snapshot/replication 기반 백업 권장 (K8s Secret 직접 저장 지양) |
| **IV 생성** | `crypto/rand`로 매 암호화마다 12 bytes 랜덤 생성 (IV 재사용 금지) |

### 13.2.1 OpenBao-first 운영 정책 (신규)

- 운영/스테이징 환경에서는 `NULLUS_ENCRYPTION_KEY`를 직접 주입하지 않고 OpenBao에서 조회한다.
- 앱은 OpenBao auth(kubernetes) 기반 short-lived credential로 키를 조회한다.
- 키 로테이션 시 `encryption_key_id`를 기준으로 점진 재암호화를 수행한다.
- 예외적으로 로컬 개발 환경에서만 `.env.dev` fallback 키를 허용한다.

### 13.3 Go 구현 예시

```go
// 암호화
func EncryptKubeconfig(plaintext []byte, key []byte) (ciphertext, iv, tag []byte, err error) {
    block, err := aes.NewCipher(key)
    if err != nil { return nil, nil, nil, err }

    aesGCM, err := cipher.NewGCM(block)
    if err != nil { return nil, nil, nil, err }

    iv = make([]byte, aesGCM.NonceSize()) // 12 bytes
    if _, err := io.ReadFull(rand.Reader, iv); err != nil { return nil, nil, nil, err }

    sealed := aesGCM.Seal(nil, iv, plaintext, nil)
    // sealed = ciphertext + tag (마지막 16 bytes)
    tagSize := aesGCM.Overhead()
    ciphertext = sealed[:len(sealed)-tagSize]
    tag = sealed[len(sealed)-tagSize:]

    return ciphertext, iv, tag, nil
}

// 복호화
func DecryptKubeconfig(ciphertext, iv, tag, key []byte) ([]byte, error) {
    block, err := aes.NewCipher(key)
    if err != nil { return nil, err }

    aesGCM, err := cipher.NewGCM(block)
    if err != nil { return nil, err }

    sealed := append(ciphertext, tag...)
    plaintext, err := aesGCM.Open(nil, iv, sealed, nil)
    if err != nil { return nil, err } // 인증 실패 = 데이터 변조

    return plaintext, nil
}
```

---

## 14. JSONB 활용 전략

### 14.1 JSONB 사용 원칙

| 원칙 | 설명 |
|------|------|
| **유연한 설정 데이터** | 스택/파이프라인 설정처럼 구조가 변할 수 있는 데이터에 JSONB 사용 |
| **조회가 잦은 필드는 컬럼화** | 자주 WHERE/ORDER BY에 사용되는 필드는 별도 컬럼으로 추출 |
| **GIN 인덱스** | JSONB 검색이 필요한 컬럼에 GIN 인덱스 적용 |
| **스냅샷 용도** | 이력 테이블의 스냅샷은 JSONB로 저장 (스키마 변경에 영향받지 않음) |
| **검증** | 애플리케이션 레이어에서 JSON Schema 검증 수행 |

### 14.2 JSONB 컬럼 목록

| 테이블 | 컬럼 | 용도 | GIN 인덱스 |
|--------|------|------|-----------|
| `stack_configs.config_json` | 스택 전체 설정 (5단계 워크플로우) | O |
| `stack_config_versions.config_snapshot` | 버전별 설정 스냅샷 | X (조회 빈도 낮음) |
| `golden_path_templates.tools_config` | 템플릿 도구 구성 | X |
| `golden_path_templates.resource_baseline` | 기본 리소스 요구량 | X |
| `compatibility_matrices.tools` | 도구별 버전 호환성 | O |
| `deployments.helm_releases` | Helm 릴리스 목록 | X |
| `deployments.rollback_stack` | 롤백 작업 스택 | X |
| `deployment_logs.metadata` | 로그 추가 컨텍스트 | X |
| `pipeline_configs.params_json` | 파이프라인 파라미터 | O |
| `pipeline_deployments.k8s_objects` | K8s 오브젝트 목록 | X |
| `pipeline_deployments.config_snapshot` | 파이프라인 설정 스냅샷 | X |
| `pipeline_templates.stages` | 파이프라인 단계 정의 | X |
| `pipeline_templates.variables` | 템플릿 변수 정의 | X |
| `alert_configs.smtp_config` | SMTP 설정 | X |
| `alert_configs.thresholds` | 알림 임계값 | X |
| `alert_history.metadata` | 알림 메타데이터 | X |
| `audit_logs.changes` | 변경 전/후 값 | X |
| `audit_logs.request_metadata` | 요청 메타데이터 | X |
| `clusters.namespaces_cache` | 네임스페이스 캐시 | X |

### 14.3 `stack_configs.config_json` 상세 구조

```json
{
  "artifacts": {
    "package_registry": {
      "tool": "gitlab",
      "helm_version": "8.7.2",
      "app_version": "17.7.2",
      "instances": 1
    },
    "source_repository": {
      "tool": "gitlab",
      "helm_version": "8.7.2",
      "app_version": "17.7.2",
      "instances": 1
    },
    "container_registry": {
      "tool": "gitlab-registry",
      "version": "17.7.2",
      "instances": 1
    },
    "storage_backend": {
      "tool": "minio",
      "helm_version": "5.3.0",
      "app_version": "2024.11.7",
      "instances": 1
    }
  },
  "pipeline": {
    "ci_platform": {
      "tool": "gitlab-ci",
      "version": "17.7.2",
      "runner_instances": 4
    },
    "cd_tool": {
      "tool": "argocd",
      "helm_version": "7.7.0",
      "app_version": "2.13.2",
      "instances": 1
    }
  },
  "monitoring": {
    "collection": {
      "tool": "prometheus",
      "helm_version": "67.0.0",
      "app_version": "3.1.0",
      "instances": 1
    },
    "visualization": {
      "tool": "grafana",
      "version": "11.4.0",
      "instances": 1
    }
  },
  "logging": {
    "collection": {
      "tool": "opentelemetry",
      "version": "0.115.0",
      "instances": 1
    },
    "search": {
      "tool": "opensearch",
      "version": "2.18.0",
      "instances": 1
    }
  },
  "resources": {
    "mode": "auto",
    "input": {
      "developers": 20,
      "concurrent_runners": 5,
      "weekly_commits": 100,
      "build_frequency": "hourly"
    },
    "calculated": {
      "cpu_cores": 12,
      "memory_gi": 24,
      "storage_gi": 180,
      "monthly_cost_usd": 150
    },
    "currency": "USD"
  },
  "custom_overrides": {}
}
```

---

## 15. 마이그레이션 전략

### 15.1 도구: golang-migrate

아키텍처 문서에서 확정된 `golang-migrate`를 사용한다.

```
migrations/
├── 000001_create_extensions.up.sql
├── 000001_create_extensions.down.sql
├── 000002_create_enums.up.sql
├── 000002_create_enums.down.sql
├── 000003_create_auth_tables.up.sql
├── 000003_create_auth_tables.down.sql
├── 000004_create_org_tables.up.sql
├── 000004_create_org_tables.down.sql
├── 000005_create_cluster_tables.up.sql
├── 000005_create_cluster_tables.down.sql
├── 000006_create_stack_tables.up.sql
├── 000006_create_stack_tables.down.sql
├── 000007_create_cicd_tables.up.sql
├── 000007_create_cicd_tables.down.sql
├── 000008_create_observability_tables.up.sql
├── 000008_create_observability_tables.down.sql
├── 000009_create_audit_logs.up.sql
├── 000009_create_audit_logs.down.sql
├── 000010_seed_rbac_policies.up.sql
├── 000010_seed_rbac_policies.down.sql
├── 000011_seed_menu_permissions.up.sql
├── 000011_seed_menu_permissions.down.sql
├── 000012_seed_golden_path_templates.up.sql
├── 000012_seed_golden_path_templates.down.sql
└── 000013_create_default_org_and_admin.up.sql
    000013_create_default_org_and_admin.down.sql
```

### 15.2 마이그레이션 정책

| 릴리스 | 정책 |
|--------|------|
| **Alpha/Beta** | 순방향(Up) 마이그레이션만 지원. 스키마 변경 시 기존 데이터 삭제 허용 |
| **v1.0 GA** | 양방향(Up/Down) 마이그레이션 필수. 롤백 테스트 자동화 |

### 15.3 마이그레이션 실행

```bash
# 마이그레이션 적용
migrate -path ./migrations -database "postgres://user:pass@localhost:5432/nullus?sslmode=disable" up

# 마이그레이션 롤백 (1단계)
migrate -path ./migrations -database "postgres://user:pass@localhost:5432/nullus?sslmode=disable" down 1

# 버전 확인
migrate -path ./migrations -database "postgres://user:pass@localhost:5432/nullus?sslmode=disable" version
```

### 15.4 CI 파이프라인 검증

매 PR마다 다음을 자동 검증한다:

1. 빈 DB에서 전체 Up 마이그레이션 성공 여부
2. 전체 Down 마이그레이션 후 빈 DB 상태 복원 여부 (v1.0 GA)
3. 시드 데이터 정합성 검증
4. JSONB 스키마 검증 (Go 단위 테스트)

---

## 16. 이력 관리 테이블 설계

### 16.1 스택 버전 스냅샷

`stack_config_versions` 테이블은 스택 설정 변경 시마다 전체 설정의 스냅샷을 저장한다.

**버전 생성 시점:**
- 스택 설정 생성 시 (version 1)
- 스택 설정 수정 시 (version 자동 증가)
- 배포 시작 시 (해당 버전으로 배포 기록)

**diff 조회:**
```sql
-- 두 버전 간 설정 차이 조회 (애플리케이션에서 json-diff 처리)
SELECT
    v1.version_number AS from_version,
    v2.version_number AS to_version,
    v1.config_snapshot AS from_config,
    v2.config_snapshot AS to_config,
    v2.change_reason,
    v2.changed_by,
    v2.created_at AS changed_at
FROM stack_config_versions v1
JOIN stack_config_versions v2
    ON v1.stack_config_id = v2.stack_config_id
WHERE v1.stack_config_id = $1
    AND v1.version_number = $2
    AND v2.version_number = $3;
```

### 16.2 파이프라인 배포 스냅샷

`pipeline_deployments` 테이블은 배포 시점의 파이프라인 설정 스냅샷과 생성된 K8s 오브젝트를 저장한다.

**롤백 지원:**
```sql
-- 특정 버전의 설정으로 롤백 (스냅샷에서 복원)
SELECT config_snapshot, k8s_objects
FROM pipeline_deployments
WHERE pipeline_config_id = $1 AND version = $2;
```

### 16.3 감사 로그와의 연계

모든 이력 변경은 `audit_logs`에도 기록되어 "누가, 언제, 무엇을, 왜" 변경했는지 추적 가능하다.

```
stack_config_versions (설정 스냅샷: "무엇이" 변경됨)
        +
audit_logs (감사 기록: "누가, 언제, 왜" 변경됨, IP/경로 포함)
        =
완전한 변경 이력 추적
```

---

## 부록

### A. 테이블 전체 목록

| # | 테이블명 | Context | 행 증가율 | 파티셔닝 |
|---|---------|---------|----------|---------|
| 1 | `users` | Auth | 낮음 | 불필요 |
| 2 | `sessions` | Auth | 중간 (TTL 만료) | 불필요 |
| 3 | `rbac_policies` | Auth | 낮음 (시드 데이터) | 불필요 |
| 4 | `menu_permissions` | Auth | 낮음 (시드 데이터) | 불필요 |
| 5 | `organizations` | Organization | 낮음 | 불필요 |
| 6 | `org_members` | Organization | 낮음 | 불필요 |
| 7 | `invite_links` | Organization | 낮음 | 불필요 |
| 8 | `org_cluster_access` | Organization | 낮음 | 불필요 |
| 9 | `clusters` | Cluster | 낮음 | 불필요 |
| 10 | `golden_path_templates` | Stack | 낮음 (시드 데이터) | 불필요 |
| 11 | `stack_configs` | Stack | 낮음 | 불필요 |
| 12 | `stack_config_versions` | Stack | 중간 | 불필요 |
| 13 | `deployments` | Stack | 중간 | 불필요 |
| 14 | `deployment_steps` | Stack | 중간 | 불필요 |
| 15 | `deployment_logs` | Stack | 높음 | 월별 파티셔닝 권장 |
| 16 | `compatibility_matrices` | Stack | 낮음 | 불필요 |
| 17 | `pipeline_templates` | CI/CD | 낮음 (시드 데이터) | 불필요 |
| 18 | `pipeline_template_versions` | CI/CD | 낮음 | 불필요 |
| 19 | `pipeline_configs` | CI/CD | 낮음 | 불필요 |
| 20 | `pipeline_deployments` | CI/CD | 중간 | 불필요 |
| 21 | `alert_configs` | Observability | 낮음 | 불필요 |
| 22 | `alert_history` | Observability | 높음 | 월별 파티셔닝 권장 |
| 23 | `audit_logs` | 공통 | 높음 | 월별 파티셔닝 권장 |

### B. 데이터 보존 정책

| 데이터 | 보존 기간 | 근거 |
|--------|----------|------|
| 감사 로그 | 1년 | 보안 운영 정책 |

---

## 17. Token Rotation 테이블 설계 (OpenBao)

OpenBao-first 정책에서 토큰 자동 갱신 상태와 이벤트를 추적하기 위한 테이블입니다.

### 17.1 `token_sources`

토큰 소스(경로/유형/상태) 메타데이터

```sql
CREATE TABLE token_sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL,
    module VARCHAR(50) NOT NULL,          -- auth | stack | cicd | observability | shared
    provider VARCHAR(50) NOT NULL,        -- keycloak | github | gitlab | harbor | slack ...
    path TEXT NOT NULL,                   -- OpenBao path
    token_type VARCHAR(30) NOT NULL,      -- lease | reissue | manual
    status VARCHAR(30) NOT NULL,          -- healthy | renew_due | renewing | rotated | failed_retryable | failed_manual | expired
    expires_at TIMESTAMPTZ,
    last_rotated_at TIMESTAMPTZ,
    next_check_at TIMESTAMPTZ,
    requires_approval BOOLEAN NOT NULL DEFAULT FALSE,
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_token_sources_org_status ON token_sources (org_id, status) WHERE deleted_at IS NULL;
CREATE INDEX idx_token_sources_next_check ON token_sources (next_check_at) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX uk_token_sources_org_provider_path ON token_sources (org_id, provider, path) WHERE deleted_at IS NULL;
```

### 17.2 `token_rotation_events`

토큰 갱신/실패/승인 이력

```sql
CREATE TABLE token_rotation_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_source_id UUID NOT NULL,
    event_type VARCHAR(30) NOT NULL,      -- renew | reissue | fail | approve | apply
    result VARCHAR(20) NOT NULL,          -- success | failure
    reason_code VARCHAR(100),
    detail_json JSONB DEFAULT '{}'::jsonb,
    trace_id VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_token_rotation_events_source
      FOREIGN KEY (token_source_id) REFERENCES token_sources(id)
);

CREATE INDEX idx_token_rotation_events_source_time ON token_rotation_events (token_source_id, created_at DESC);
CREATE INDEX idx_token_rotation_events_result ON token_rotation_events (result, created_at DESC);
```

### 17.3 상태 전이 규칙

- `healthy -> renew_due -> renewing -> rotated`
- `renewing -> failed_retryable -> renewing` (백오프 재시도)
- `renewing -> failed_manual -> renewing` (승인 후)
- `renew_due/renewing/failed_* -> expired` (만료 초과)

### 17.4 보안/저장 원칙

- `token_sources`/`token_rotation_events`에는 원문 토큰을 저장하지 않습니다.
- OpenBao path, 만료 시각, 실패 원인 코드, 감사 메타데이터만 저장합니다.
- 원문 비밀값은 OpenBao에만 존재하며 DB는 상태 추적 용도로만 사용합니다.
| 배포 로그 | 6개월 | 디버깅 용도 |
| 알림 이력 | 6개월 | 운영 분석 |
| 세션 데이터 | 만료 후 7일 | 자동 정리 |
| 소프트 삭제 데이터 | 90일 후 완전 삭제 | 복구 기간 |
| 스택 버전 스냅샷 | 무기한 | 롤백 지원 |

> 구현 정책 메모: 스택 삭제 API는 hard delete 대신 soft delete(`deleted_at`)를 사용해 스택 이력 스냅샷을 보존한다.

### C. 성능 고려사항

| 항목 | 전략 |
|------|------|
| **Connection Pool** | pgx pool, max_conns=25, min_conns=5 |
| **읽기 부하 분산** | v1.0에서는 단일 인스턴스, 향후 Read Replica 고려 |
| **JSONB 조회 최적화** | 자주 필터링하는 키에 GIN 인덱스 또는 Expression Index |
| **로그 테이블** | 파티셔닝 + 오래된 파티션 자동 DROP |
| **Vacuum** | autovacuum 기본 설정 유지, deployment_logs에 aggressive 설정 |
| **백업** | 일 1회 pg_dump, RPO 24시간, RTO 4시간 |
