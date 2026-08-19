# =============================================================================
# Nullus Jenkins — 플러그인을 미리 넣은 이미지
# =============================================================================
# 기본 차트는 파드가 뜰 때마다 init 컨테이너가 updates.jenkins.io 에서 플러그인을
# 받는다. 그 방식은 두 곳에서 무너진다.
#
#   - 설치 시간이 네트워크에 좌우된다. SSO 용 oic-auth(의존 플러그인 다수)를
#     더하자 의존성 해석에 plugin-versions.json(대용량)이 필요해졌고, 준비 검사
#     600초를 넘겨 설치가 통째로 실패했다
#   - 에어갭에서는 아예 나갈 수 없다
#
# 그래서 플러그인을 빌드 시점에 굽는다. 느린 다운로드가 매 설치에서 한 번의
# 빌드로 옮겨 가고, 그 결과물은 에어갭 번들에도 그대로 실린다.
#
# 빌드:
#   docker build -t ghcr.io/cloud-nullus/nullus-jenkins:<태그> deploy/images/jenkins
# =============================================================================
ARG JENKINS_VERSION=2.568.2
FROM jenkins/jenkins:${JENKINS_VERSION}-jdk21

# 플러그인 목록의 단일 출처다. 여기와 차트 values 가 갈라지면 차트가 런타임에
# 다시 받으려 하므로, values 는 installPlugins 를 끄고 이 목록만 신뢰한다.
#
#   kubernetes            k8s 에이전트로 빌드를 돌린다
#   workflow-aggregator   파이프라인 기본 묶음
#   git / gitea           SCM 연동
#   configuration-as-code JCasC — 자격증명과 보안 영역을 코드로 설정한다
#   pipeline-stage-view   단계별 실행 결과를 /wfapi 로 내보낸다
#   oic-auth              OIDC 로그인(SSO)
ARG JENKINS_PLUGINS="kubernetes workflow-aggregator git gitea configuration-as-code pipeline-stage-view oic-auth"

RUN jenkins-plugin-cli --verbose --plugins ${JENKINS_PLUGINS}
