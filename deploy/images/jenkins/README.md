# Nullus Jenkins 이미지

플러그인을 빌드 시점에 구운 Jenkins 이미지다.

기본 차트는 파드 기동마다 `updates.jenkins.io` 에서 플러그인을 받는데, SSO 용
`oic-auth` 를 더하면서 의존성 해석에 대용량 메타데이터가 필요해져 준비 검사
600초를 넘겼다(설치 실패). 에어갭에서는 애초에 불가능하다.

## 빌드

```bash
./scripts/build-jenkins-image.sh                 # 빌드만
./scripts/build-jenkins-image.sh --kind-load     # 빌드 후 kind 클러스터에 적재
```

## 플러그인을 추가할 때

`Dockerfile` 의 `JENKINS_PLUGINS` 만 고친다. 차트 values 는 `installPlugins` 를
꺼 두므로 런타임에 다시 받지 않는다 — 두 목록이 갈라지면 그 차이만큼 다시
네트워크를 타게 되어 같은 실패가 돌아온다.
