# Zadara Cloud PoC — 구축/배포 실행 로그 (2026-09-13)

작업 시각: KST 00:50–01:45 (2026-09-13). 대상: [OpenTofu 런북](opentofu/README.md) 기준
m1+w1 신규 인프라 apply → Kubespray → Nullus 배포. 상세 정적 검증은
[opentofu/VALIDATION.md](opentofu/VALIDATION.md) 참고, 배포 절차는 [README](README.md).

## 요약

기존 node-10/11/20/21(platform/develop 2클러스터)을 백업 없이 폐기하고, OpenTofu로
m1(control-plane+worker+bastion)/w1(worker) 단일 클러스터를 새로 만들어 Nullus를
배포했다. `https://nullus.io/`가 200을 반환하는 상태까지 확인했다. 브라우저 로그인
클릭 흐름은 미검증.

## 결과 환경

- m1: z2.xlarge 4vCPU/8GB/100GB, 사설 10.20.0.10, 공인 **121.78.39.241**,
  control-plane+etcd+kube_node(taint 없음)+bastion.
- w1: z2.4xlarge 16vCPU/32GB/300GB, 사설 10.20.1.10, private 서브넷(NAT egress),
  SSH는 m1 ProxyCommand로만.
- Kubespray v2.30.0(`f4ccdb5`), Kubernetes v1.34.3, containerd 2.2.1, Calico VXLAN,
  Ubuntu 24.04.3, kube-proxy ipvs. Pod CIDR 10.233.64.0/18, Service CIDR 10.233.0.0/18.
- 애드온: local-path-provisioner v0.0.31(default SC), ingress-nginx 4.15.1
  NodePort 30080/30443(w1), cert-manager v1.16.2 + ClusterIssuer `letsencrypt`(HTTP-01),
  Certificate `nullus-wildcard`(SAN: nullus.io/www/auth, secret `nullus-wildcard-tls`,
  Let's Encrypt YR1, 만료 2026-12-11).
- 80/443 진입: m1 `nullus-web-expose.service` → iptables nat `NULLUS-WEB`
  (MARK 0x4000 → REDIRECT 80→30080/443→30443).
- Nullus: helm release `nullus` ns `nullus`, chart `nullus-0.5.0` rev 1, values
  `deploy/csp/zadara/values-zadara.yaml` 무수정, `--set secrets.dbPassword
  --set postgresql.auth.password --set secrets.encryptionKey`(각 `openssl rand -hex 16`).
  시크릿은 m1의 `~/.nullus-secrets`(0600)에만 존재. 마이그레이션 Job `nullus-migrate`
  Complete(1/1, 8s). PVC `data-nullus-postgresql-0` 20Gi,
  `data-nullus-keycloak-postgresql-0` 8Gi Bound.
- Keycloak: realm `nullus`, client `nullus-app`(public, redirectUris
  `https://nullus.io/*`, `http://localhost:5173/*`), 계정
  admin@nullus.io/devops@nullus.io/dev@nullus.io 기본 비밀번호 `nullus123!`(변경 권고).
  OSS 클라이언트 grafana/argocd/harbor `*-dev-secret`.

## 검증 결과

| 확인 | 결과 |
|---|---|
| `https://nullus.io/` | 200 |
| `https://nullus.io/healthz` | 200 |
| `https://nullus.io/config.js` | 200, `oidcAuthority=https://auth.nullus.io/realms/nullus`, `clientId=nullus-app` |
| OIDC discovery | 200 |
| authorize 엔드포인트 | "Sign in to nullus" 로그인 페이지 HTML 반환 |
| 브라우저 클릭 로그인 | 미확인 (Claude in Chrome 미연결) |

## 실패/우회 기록 (시간순)

1. **사전 검증 통과, 실행은 보류했다가 재개.** `tofu fmt`/`validate`/`test` 15 pass,
   Python 단위 테스트 3 pass, `tofu plan` 31 add(신규 인프라만, 기존 4대 미포함).
   기존 4대가 잠정 quota 전량 사용 중이고 PVC 실데이터 백업이 없어 사용자에게
   선택지를 제시했고, "기존 것 전부 삭제하고 신규로 간다"로 결정됨.
2. **zsh 변수 word-splitting.** `P="aws ..."; $P ec2 ...` 형태가 zsh에서
   `command not found`로 깨짐. 전체 명령을 변수 치환 없이 풀어 씀.
3. **apply 1차 실패 (18/31 생성 후).** `aws_security_group.node`의 `tags`에서
   `400 InvalidParameterValue: Invalid format for tags: [u'Name=nullus-poc-v2-node']`.
   별도 `create-tags`는 exit 0이지만 이후 `describe-vpcs`의 Tags가 `[]`로 돌아옴 — 이
   플랫폼은 태그를 저장하지 않는다. **수정(D7):** SG의 `tags` 제거.
4. **apply 2차 실패 (SG/rule 9개 생성 후).** `aws_instance`의
   `associate_public_ip_address=false` 지정 시 `400 Network details contain
   unsupported params AssociatePublicIpAddress`. **수정(D8):** 해당 속성 제거, 공인 IP는
   `aws_eip.node`로만 붙인다. 3차 apply로 나머지 4개 생성, 합계 31개 완료.
5. **Ansible non-blocking IO.** 파이프 실행 환경에서
   `ERROR: Ansible requires blocking IO on stdin/stdout/stderr`. 우회:
   `... </dev/null >log 2>&1`로 표준 입출력을 파일/널로 고정.
6. **Kubespray ignored 4건 (정상 동작).** 최초 설치 시 `etcd --version`,
   `calicoctl get felixconfig/ippool/bgpconfig` 조회가 `...ignoring`으로 표시됨.
   RECAP: m1 ok=667 failed=0 ignored=4, w1 ok=417 failed=0 — 실패 아님.
7. **Helm 1차 실패.** `YAML parse error on
   nullus/templates/keycloak-theme-configmap.yaml: control characters are not allowed`.
   원인: macOS `tar`가 AppleDouble `._*` 메타파일을 함께 전송했고, 차트의
   `.Files.Glob`이 그 바이너리 파일을 텍스트로 렌더링함. 로컬 `helm template`은
   정상이라 처음엔 재현이 안 됐다. **수정:** m1에서 `find . -name '._*' -delete` 실행,
   이후 전송은 `COPYFILE_DISABLE=1 tar --exclude='._*' ...`로 고정.
8. **80/443 외부 접속 무응답.** 보안 그룹 규칙은 있고 m1의 NULLUS-WEB 체인 카운터도
   증가했지만 응답이 없었다. tcpdump로 확인: vxlan.calico가 원본 src IP를 그대로 w1
   파드에 전달하고, w1이 직접(NAT 경유) 응답하며 비대칭 라우팅이 발생. 원인: REDIRECT가
   nat PREROUTING 체인을 그 자리에서 종료시켜 KUBE-SERVICES의 masquerade 마크(0x4000)를
   건너뜀. **수정:** REDIRECT 규칙 앞에 `MARK --set-xmark 0x4000/0x4000`을 추가
   (`expose-web.sh`가 이미 문서화한 규칙과 동일). 이후 http 308 / https 200으로 정상화.
9. **로컬호스트 NodePort 테스트 000.** `curl 127.0.0.1:30080`은 ipvs 모드에서 실패하지만
   `curl 10.20.0.10:30080`은 308 정상 — 진단 시 loopback이 아니라 노드 IP로 시험할 것.
10. **`tofu apply` 4차(web_cidrs 추가) 8분 이상 멈춤.** SG rule 2개는 1~2초에 생성됐지만
    이어서 `aws_instance` m1/w1이 "Still modifying..."에서 멈춤. 원인: plan이
    `root_block_device.delete_on_termination`을 `true→false`로, root 볼륨 태그 드리프트를
    맞추려 시도했는데, zCompute가 생성 시 `false` 지정을 무시하고 항상 `true`로 만들어
    `ModifyInstanceAttribute`가 끝나지 않는 무한 대기가 됨. 프로세스를 중단하고
    **수정(D9):** `delete_on_termination=true`로 정정, root 볼륨 태그는
    `lifecycle.ignore_changes=[root_block_device[0].tags]`로 무시. 이후 `tofu plan` =
    "No changes."
11. **`setup-keycloak-realm.sh` 1차 실패.** `업스트림 스크립트를 찾지 못했습니다:
    ~/nullus/scripts/setup-keycloak.sh` — `scripts/` 디렉토리를 m1에 전송하지 않았기
    때문. 전송 후 재실행하여 성공.
12. **Claude in Chrome 미연결로 브라우저 로그인 클릭 검증 생략.** authorize 엔드포인트가
    로그인 페이지 HTML을 반환하는 것으로 대체 확인.
13. **DNS 갱신 지연.** 배포 직후 `nullus.io`는 구 EIP `121.78.39.184`를 계속 가리키고
    있었다. 사용자가 Spaceship에서 121.78.39.241로 갱신, `dig @1.1.1.1 nullus.io`로 확인.

## 폐기된 자원

- node-10/11/20/21 (platform + develop 2클러스터), 각 100GiB 볼륨 4개(인스턴스 terminate와
  함께 자동 삭제, `delete_on_termination=true`), EIP `eipalloc-bf54f437`(121.78.39.184,
  detach 상태로 남아 수동 release 필요).
- PVC 11개(nullus-devsecops-stack: harbor/gitea/jenkins/minio 500Gi/openbao/postgresql,
  nullus: keycloak-postgresql 8Gi/postgresql 20Gi), Helm release 12개 — **백업 없이
  폐기**(사용자 결정, §요약 참고).
- 보존: CoreDNS 서비스 VM 2대(zCompute 기본 VPC 소속, msit.VPC_DNS) — 이 PoC와 무관해
  건드리지 않음.

## 남은 위험 (README §12와 동일, 참조용 요약)

시크릿이 m1에만 있고 백업이 없음. local-path라 노드 재생성 시 데이터 소실. TLS는
와일드카드가 아니라 SAN 방식(호스트 추가마다 갱신 필요). CD의 ZADARA_HOST/호스트키
시크릿이 구 IP 기준일 수 있음. Keycloak 기본 비밀번호 미변경. quota 상한 미확인
(`max-instances=20`만 확인, vCPU/RAM 한도 아님).
