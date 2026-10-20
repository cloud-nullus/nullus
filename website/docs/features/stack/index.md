# 데브섹옵스 스택

검증된 OSS 조합을 Kubernetes 클러스터에 설치하고 관리하는 영역입니다.
모든 API 는 `/api/v1/stacks/**` 아래에 있고 **admin·devops** 만 호출할 수 있습니다.

<div class="nullus-cards" markdown>

[**스택 템플릿**<span>검증된 조합을 골라 설치로 들어간다.</span>](templates.md)

[**스택 설치**<span>탭 10개를 채우고 배포한다.</span>](install.md)

[**스택 목록**<span>설치된 스택의 상태·워크로드·설정·취약점을 본다.</span>](list.md)

[**스택 이력**<span>변경 내역을 비교하고 되돌린다.</span>](history.md)

[**스택 버전**<span>도구 버전 호환성 매트릭스.</span>](versions.md)

[**OSS 기본 리소스**<span>도구별 CPU·메모리 기본값.</span>](oss-defaults.md)

[**설치 OSS CVE 대응**<span>설치된 OSS 의 CVE 를 알리고 고치는 흐름과 기한.</span>](cve-response.md)

</div>

## 설치까지의 흐름

```mermaid
graph LR
  T[스택 템플릿] -->|이 템플릿 사용| I[스택 설치<br/>탭 10개]
  I -->|배포| D[배포 진행·로그]
  D --> L[스택 목록]
  L --> H[스택 이력]
```

1. **스택 템플릿** 에서 조합을 고르고 `이 템플릿 사용` 을 누릅니다
2. **스택 설치** 로 넘어가 스택 이름·클러스터·네임스페이스를 채웁니다
3. `배포` 를 누르면 **배포 진행 화면**(`/stack/deploy/:id`)으로 이동해 로그를 실시간으로 봅니다
4. 끝난 스택은 **스택 목록** 에서 관리합니다

!!! note "스택 설치는 사이드바에 없습니다"
    독립 메뉴를 두지 않고 템플릿에서 이어 들어가는 흐름입니다. 주소로 직접
    `/stack/install` 을 열 수도 있습니다.

## devops 가 설치할 때 막히는 지점이 있습니다

!!! warning "설치 위자드는 admin 전용 API 를 부릅니다"
    `/stack/install` 은 **admin·devops** 모두 열 수 있지만, 화면이 채워야 하는
    데이터 가운데 넷이 **admin 전용** 경로에서 옵니다.

    | 위자드가 쓰는 것 | 부르는 API | 권한 |
    |------------------|-----------|------|
    | 대상 클러스터 목록 | `GET /api/v1/admin/clusters` | admin |
    | 네임스페이스 목록 | `GET /api/v1/admin/clusters/{id}/namespaces` | admin |
    | StorageClass 목록 | `GET /api/v1/admin/clusters/{id}/storage-classes` | admin |
    | 조직 리소스 프로파일 | `GET /api/v1/admin/organization/resource-profiles` | admin |

    인증을 켠 환경에서 devops 로 들어가면 이 드롭다운들이 403 으로 비어 있을
    것으로 보입니다. 코드 배선으로 확인한 내용이고 실행으로 재현하지는
    않았습니다.
