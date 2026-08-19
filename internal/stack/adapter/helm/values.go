package helm

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/cloud-nullus/draft/internal/stack/domain"
)

// defaultStackNamespace 는 스택 네임스페이스가 정해지지 않았을 때의 기본값이다.
// 설치 시점에는 valuesForStep 이 실제 네임스페이스로 다시 덮어쓴다.
// 안내 경로(연결정보)도 같은 값을 봐야 하므로 domain 의 상수를 그대로 쓴다.
const defaultStackNamespace = domain.DefaultStackNamespace

// Jenkins 이미지. 플러그인을 빌드 시점에 구운 것이라 런타임 다운로드가 없다.
// 태그는 베이스 Jenkins 버전과 같게 둔다 — 갈라지면 JCasC 스키마가 안 맞을 수 있다.
// 빌드: ./scripts/build-jenkins-image.sh
const (
	jenkinsImageRegistry = "ghcr.io"
	// 차트가 registry 와 이어 붙이므로 여기에 레지스트리를 넣으면 안 된다.
	// 넣으면 ghcr.io/ghcr.io/... 가 되어 파드가 ImagePullBackOff 로 멈춘다.
	jenkinsImageRepository = "cloud-nullus/nullus-jenkins"
	jenkinsImageTag        = "2.568.2"
)

func DefaultValues(stepName string) map[string]any {
	switch stepName {
	case "installing_cert_manager":
		return map[string]any{
			"installCRDs": true,
			"resources": map[string]any{
				"requests": map[string]any{
					"cpu":    "500m",
					"memory": "512Mi",
				},
				"limits": map[string]any{
					"cpu":    "1",
					"memory": "1Gi",
				},
			},
			"webhook": map[string]any{
				"resources": map[string]any{
					"requests": map[string]any{
						"cpu":    "250m",
						"memory": "256Mi",
					},
					"limits": map[string]any{
						"cpu":    "500m",
						"memory": "512Mi",
					},
				},
			},
			"cainjector": map[string]any{
				"resources": map[string]any{
					"requests": map[string]any{
						"cpu":    "250m",
						"memory": "256Mi",
					},
					"limits": map[string]any{
						"cpu":    "500m",
						"memory": "512Mi",
					},
				},
			},
		}
	case "installing_metrics_server":
		return map[string]any{
			"args": []string{
				"--kubelet-insecure-tls",
				"--kubelet-preferred-address-types=InternalIP,Hostname,ExternalIP",
			},
			"resources": map[string]any{
				"requests": map[string]any{
					"cpu":    "250m",
					"memory": "256Mi",
				},
				"limits": map[string]any{
					"cpu":    "500m",
					"memory": "512Mi",
				},
			},
		}
	case "installing_postgresql":
		return map[string]any{
			"architecture": "standalone",
			// 이미지를 고정하지 않으면 차트 기본값(bitnami/postgresql:latest)을
			// 쓰게 되고, 그 태그는 롤링이라 설치 시점마다 내용이 달라진다.
			// bitnamilegacy 는 비공식 저장소로 분류돼 allowInsecureImages 가 필요하다.
			"global": map[string]any{
				"security": map[string]any{
					"allowInsecureImages": true,
				},
			},
			"image": map[string]any{
				"registry":   "docker.io",
				"repository": "bitnamilegacy/postgresql",
				"tag":        "17.6.0-debian-12-r4",
			},
			// 비밀번호는 values 에 넣지 않는다. provisioning_secrets 가 생성해
			// OpenBao 에 기록하고 ESO 가 복제한 Secret 을 참조한다.
			"auth": map[string]any{
				"username":       domain.PostgresAppUser,
				"database":       domain.PostgresAppDatabase,
				"existingSecret": ProvisionedPostgresSecret,
				"secretKeys": map[string]any{
					"userPasswordKey":        domain.PostgresPasswordKey,
					"adminPasswordKey":       "postgres-password",
					"replicationPasswordKey": "replication-password",
				},
			},
			"primary": map[string]any{
				"resources": map[string]any{
					"requests": map[string]any{
						"cpu":    "1",
						"memory": "2Gi",
					},
					"limits": map[string]any{
						"cpu":    "2",
						"memory": "4Gi",
					},
				},
				"persistence": map[string]any{
					"enabled": true,
					"size":    "20Gi",
				},
			},
		}
	case "installing_minio":
		return map[string]any{
			"mode": "standalone",
			// rootUser/rootPassword 대신 프로비저닝된 Secret 을 참조한다.
			"existingSecret": ProvisionedMinIOSecret,
			"ingress": map[string]any{
				"enabled": false,
			},
			"consoleIngress": map[string]any{
				"enabled": false,
			},
			"resources": map[string]any{
				"requests": map[string]any{
					"cpu":    "500m",
					"memory": "512Mi",
				},
				"limits": map[string]any{
					"cpu":    "1",
					"memory": "2Gi",
				},
			},
		}
	case "installing_harbor":
		return map[string]any{
			// 인그레스는 게이트웨이가 담당하므로 차트는 Service 만 낸다.
			// 기본값 ingress 로 두면 클러스터에 없는 IngressClass 를 요구해
			// 설치가 Pending 으로 멈춘다.
			"expose": map[string]any{
				"type": "clusterIP",
				"tls": map[string]any{
					"enabled": false,
				},
			},
			// externalURL 은 valuesForStep 이 accessDomain 으로 다시 채운다.
			// 여기 값은 도메인을 모를 때의 폴백이다.
			"externalURL": fmt.Sprintf("http://%s.%s.svc.cluster.local", domain.HarborServiceName, defaultStackNamespace),
			// 관리자 비밀번호는 values 에 넣지 않는다. provisioning_secrets 가
			// 만든 Secret 을 참조한다.
			"existingSecretAdminPassword":    domain.HarborAdminSecret,
			"existingSecretAdminPasswordKey": domain.HarborAdminPassKey,
			// Trivy 는 기동 시 취약점 DB를 통째로 내려받는다. 로컬/소규모
			// 클러스터에서 설치를 가장 자주 실패시키는 지점이라 기본은 끈다.
			"trivy": map[string]any{
				"enabled": false,
			},
			"persistence": map[string]any{
				"enabled": true,
			},
			"core": map[string]any{
				"resources": map[string]any{
					"requests": map[string]any{"cpu": "100m", "memory": "256Mi"},
					"limits":   map[string]any{"cpu": "1", "memory": "1Gi"},
				},
			},
			"registry": map[string]any{
				"registry": map[string]any{
					"resources": map[string]any{
						"requests": map[string]any{"cpu": "100m", "memory": "256Mi"},
						"limits":   map[string]any{"cpu": "1", "memory": "1Gi"},
					},
				},
			},
			"database": map[string]any{
				"type": "internal",
			},
			"redis": map[string]any{
				"type": "internal",
			},
		}
	case "installing_nexus":
		return map[string]any{
			// 차트 기본 이름은 {release}-nexus-repository-manager 라 길다.
			// 연결정보와 CI 가 안내하는 주소를 짧게 유지하려고 맞춘다.
			"fullnameOverride": domain.NexusServiceName,
			"nexus": map[string]any{
				"env": []any{
					// 기본 JVM 값(2703M)은 소규모 클러스터에서 스케줄되지 않는다.
					map[string]any{
						"name":  "INSTALL4J_ADD_VM_PARAMS",
						"value": "-Xms1200M -Xmx1200M -XX:MaxDirectMemorySize=1200M -Djava.util.prefs.userRoot=/nexus-data/javaprefs",
					},
					// 초기 비밀번호를 무작위로 두고, provisioning_nexus 가 그것으로
					// 로그인해 프로비저닝한 비밀번호로 바꾼다. false 로 두면
					// 잘 알려진 기본 비밀번호가 그대로 남는다.
					map[string]any{"name": "NEXUS_SECURITY_RANDOMPASSWORD", "value": "true"},
				},
				// properties.override 는 켜지 않는다.
				//
				// 켜면 차트가 /nexus-data/etc/nexus.properties 를 subPath 로
				// 마운트하는데, 쿠버네티스가 그 상위 디렉터리 /nexus-data/etc 를
				// root 소유로 만든다. 컨테이너는 UID 200 으로 돌아 그 안에
				// logback 디렉터리를 만들지 못하고 기동 직후 죽는다(fsGroup 으로도
				// 해결되지 않는다 — subPath 상위 경로는 fsGroup 적용 대상이 아니다).
				//
				// 저장소 생성은 provisioning_nexus 가 REST API 로 하므로
				// nexus.scripts.allowCreation 도 필요 없다.
				"resources": map[string]any{
					"requests": map[string]any{"cpu": "200m", "memory": "1536Mi"},
					"limits":   map[string]any{"cpu": "2", "memory": "3Gi"},
				},
			},
			"ingress": map[string]any{
				"enabled": false,
			},
			"persistence": map[string]any{
				"enabled":     true,
				"storageSize": "20Gi",
			},
		}
	case "installing_jenkins":
		// Jenkins 는 GitLab CI 와 달리 실행기를 별도 차트로 세우지 않는다.
		// kubernetes 플러그인이 빌드마다 agent 파드를 띄우므로 컨트롤러만 세운다.
		//
		// admin 자격증명은 values 에 평문으로 두지 않는다 — provisioning_secrets 가
		// OpenBao 에 만들어 ESO 로 동기화한 Secret 을 가리킨다. 차트가 읽는 키
		// 이름은 controller.admin.userKey/passwordKey 기본값과 같아야 한다.
		return map[string]any{
			"controller": map[string]any{
				"admin": map[string]any{
					"existingSecret": domain.JenkinsAdminSecret,
					"userKey":        domain.JenkinsAdminUserKey,
					"passwordKey":    domain.JenkinsAdminPasswordKey,
				},
				// gitea 플러그인이 Gitea organization/multibranch 스캔을 제공한다.
				// 이게 없으면 리포를 만들어도 Jenkins 가 Jenkinsfile 을 찾지 못한다.
				// (이 플러그인이 Jenkins 2.528.3 이상을 요구하므로 차트 버전을
				//  임의로 내릴 수 없다 — domain.JenkinsChartVersion 주석 참고)
				// 플러그인은 이미지에 구워져 있다(deploy/images/jenkins).
				//
				// 기본 차트는 파드가 뜰 때마다 updates.jenkins.io 에서 받는데,
				// SSO 용 oic-auth 를 더하자 의존성 해석에 대용량 메타데이터가
				// 필요해져 준비 검사 600초를 넘겼고 설치가 통째로 실패했다.
				// 에어갭에서는 애초에 나갈 수 없다.
				//
				// 목록의 단일 출처는 Dockerfile 의 JENKINS_PLUGINS 다. 여기서
				// 다시 지정하면 그 차이만큼 런타임에 또 받게 되어 같은 실패가
				// 돌아온다.
				"installPlugins": false,
				"image": map[string]any{
					"registry":   jenkinsImageRegistry,
					"repository": jenkinsImageRepository,
					"tag":        jenkinsImageTag,
					// 고정 태그다. Always 면 노드에 적재해 둔 이미지를 두고
					// 레지스트리를 쳐서, 아직 published 되지 않은 지금은 실패한다.
					"pullPolicy": "IfNotPresent",
				},
				// 서비스는 게이트웨이 라우트가 앞단을 맡으므로 ClusterIP 로 둔다.
				"serviceType": "ClusterIP",
				"servicePort": domain.JenkinsServicePort,
				// ESO 가 동기화한 Gitea 자격증명을 컨트롤러에 마운트한다.
				// JCasC 는 {name}-{keyName} 으로 참조하므로 아래 configScripts 의
				// ${nullus-gitea-credentials-username} 과 짝이 맞아야 한다.
				"additionalExistingSecrets": []any{
					map[string]any{"name": domain.GiteaAdminSecret, "keyName": domain.GiteaAdminUserKey},
					map[string]any{"name": domain.GiteaAdminSecret, "keyName": domain.GiteaAdminPasswordKey},
				},
				"JCasC": map[string]any{
					"defaultConfig": true,
					// multibranch job 이 private Gitea 리포를 스캔하려면 credential
					// 객체가 필요하다. agent 파드의 env 로는 안 된다 — 스캔은
					// 컨트롤러가 하고 Jenkins 는 실제 credential 을 요구한다.
					//
					// id 는 job 설정이 참조하는 이름과 같아야 한다
					// (cicd/usecase 의 giteaCredentialID). 갈라지면 job 은 만들어지되
					// 브랜치를 하나도 찾지 못한다.
					"configScripts": map[string]any{
						"nullus-gitea-credentials": jenkinsGiteaCredentialJCasC(),
					},
				},
			},
			// 컨트롤러 홈은 job 설정과 빌드 이력을 담는다. 비영속으로 두면
			// 파드가 재시작될 때마다 multibranch job 이 사라진다.
			"persistence": map[string]any{
				"enabled": true,
				"size":    "20Gi",
			},
		}
	case "installing_gitea":
		// Gitea 는 소스 저장소만 담당한다. GitLab 과 달리 CI 도 레지스트리도
		// 겸하지 않으므로 이 스택은 Jenkins·Harbor 를 따로 세운다.
		//
		// 차트 기본값은 postgresql-ha 와 valkey-cluster 를 함께 올린다. 그대로 두면
		// 스택 안에 두 번째 데이터베이스와 캐시 클러스터가 생겨 리소스를 두 배로
		// 먹는다 — 스택이 이미 세운 PostgreSQL 을 가리키고 나머지는 끈다.
		return map[string]any{
			// Gitea 는 RWO 볼륨 하나에 leveldb 큐 락을 잡는다. 차트 기본값인
			// RollingUpdate(maxUnavailable=0)로 두면 재배포가 교착된다 — 새 파드는
			// 옛 파드가 쥔 락 때문에 뜨지 못하고, 옛 파드는 새 파드가 Ready 가
			// 되어야 내려가므로 영원히 안 내려간다. 첫 설치는 되지만 values 를
			// 바꾸는 모든 재배포가 멈춘다.
			"strategy":       map[string]any{"type": "Recreate"},
			"postgresql-ha":  map[string]any{"enabled": false},
			"postgresql":     map[string]any{"enabled": false},
			"valkey-cluster": map[string]any{"enabled": false},
			"valkey":         map[string]any{"enabled": false},
			"service": map[string]any{
				"http": map[string]any{
					"port": domain.GiteaServicePort,
				},
			},
			"gitea": map[string]any{
				"admin": map[string]any{
					// 비밀번호를 values 에 평문으로 두지 않는다. provisioning_secrets 가
					// OpenBao 에 만들어 ESO 로 동기화한 Secret 을 가리킨다.
					// 차트는 이 Secret 안에서 username / password 키를 읽는다.
					"existingSecret": domain.GiteaAdminSecret,
					// keepUpdated 는 매 기동마다 비밀번호를 Secret 값으로 되돌린다.
					// 회전이 실제로 반영되려면 이 모드여야 한다.
					"passwordMode": "keepUpdated",
				},
				"config": map[string]any{
					"database": map[string]any{
						"DB_TYPE": "postgres",
						"HOST": fmt.Sprintf("%s.%s.svc.cluster.local:%d",
							domain.PostgresServiceName, defaultStackNamespace, domain.PostgresServicePort),
						"NAME": domain.PostgresAppDatabase,
						"USER": domain.PostgresAppUser,
					},
					// 캐시·큐를 껐으므로 Gitea 내부 구현을 쓴다. 그대로 두면
					// 존재하지 않는 valkey 주소로 붙으려다 기동에 실패한다.
					"cache":   map[string]any{"ADAPTER": "memory"},
					"queue":   map[string]any{"TYPE": "level"},
					"session": map[string]any{"PROVIDER": "file"},
				},
				// 비밀번호는 config 에 적지 않고 Secret 에서 환경변수로 주입한다.
				"additionalConfigFromEnvs": []any{
					map[string]any{
						"name": "GITEA__database__PASSWD",
						"valueFrom": map[string]any{
							"secretKeyRef": map[string]any{
								"name": ProvisionedPostgresSecret,
								"key":  domain.PostgresPasswordKey,
							},
						},
					},
				},
			},
		}
	case "installing_gitlab":
		return map[string]any{
			"postgresql": map[string]any{
				"install": false,
			},
			"global": map[string]any{
				"edition": "ce",
				"minio": map[string]any{
					"enabled": false,
				},
				"psql": map[string]any{
					"host":     fmt.Sprintf("%s.%s.svc.cluster.local", domain.PostgresServiceName, defaultStackNamespace),
					"port":     domain.PostgresServicePort,
					"database": domain.PostgresAppDatabase,
					"username": domain.PostgresAppUser,
					// existingSecret 으로 설치되는 PostgreSQL 차트는 자기 이름의
					// Secret 을 만들지 않으므로 프로비저닝된 Secret 을 가리킨다.
					"password": map[string]any{
						"useSecret": true,
						"secret":    ProvisionedPostgresSecret,
						"key":       domain.PostgresPasswordKey,
					},
				},
				"hosts": map[string]any{
					"domain": "nullus.internal",
					"https":  false,
				},
				"ingress": map[string]any{
					"enabled":              false,
					"configureCertmanager": false,
				},
			},
			"nginx-ingress": map[string]any{
				"enabled": false,
			},
			"gitlab": map[string]any{
				"webservice": map[string]any{
					"ingress": map[string]any{
						"enabled": false,
					},
					"readinessProbe": map[string]any{
						"initialDelaySeconds": 90,
						"periodSeconds":       10,
						"timeoutSeconds":      5,
						"failureThreshold":    18,
					},
					"livenessProbe": map[string]any{
						"initialDelaySeconds": 180,
						"periodSeconds":       20,
						"timeoutSeconds":      10,
						"failureThreshold":    6,
					},
				},
				"sidekiq": map[string]any{
					"readinessProbe": map[string]any{
						"initialDelaySeconds": 120,
						"periodSeconds":       10,
						"timeoutSeconds":      5,
						"failureThreshold":    18,
					},
					"livenessProbe": map[string]any{
						"initialDelaySeconds": 240,
						"periodSeconds":       20,
						"timeoutSeconds":      10,
						"failureThreshold":    6,
					},
				},
				"kas": map[string]any{
					"ingress": map[string]any{
						"enabled": false,
					},
				},
			},
			"registry": map[string]any{
				"ingress": map[string]any{
					"enabled": false,
				},
			},
			"certmanager": map[string]any{
				"install": false,
			},
			"certmanager-issuer": map[string]any{
				"enabled": false,
			},
			"gitlab-runner": map[string]any{
				"install": false,
			},
			"redis": map[string]any{
				"image": map[string]any{
					"repository": "bitnamilegacy/redis",
					"tag":        "7.4.2-debian-12-r0",
				},
				"master": map[string]any{
					"resources": map[string]any{
						"requests": map[string]any{
							"cpu":    "250m",
							"memory": "512Mi",
						},
						"limits": map[string]any{
							"cpu":    "500m",
							"memory": "1Gi",
						},
					},
					"readinessProbe": map[string]any{
						"enabled":             true,
						"initialDelaySeconds": 30,
						"periodSeconds":       10,
						"timeoutSeconds":      5,
						"failureThreshold":    12,
					},
					"livenessProbe": map[string]any{
						"enabled":             true,
						"initialDelaySeconds": 60,
						"periodSeconds":       15,
						"timeoutSeconds":      10,
						"failureThreshold":    8,
					},
				},
				"metrics": map[string]any{
					"image": map[string]any{
						"repository": "bitnamilegacy/redis-exporter",
						"tag":        "1.76.0-debian-12-r0",
					},
				},
			},
		}
	case "installing_argocd":
		return map[string]any{
			"crds": map[string]any{
				"install": true,
			},
			"configs": map[string]any{
				"params": map[string]any{
					"server.insecure": "true",
				},
				"secret": map[string]any{
					"extra": map[string]any{
						"server.secretkey": randomArgoCDServerSecretKey(),
					},
				},
			},
			"server": map[string]any{
				"ingress": map[string]any{
					"enabled": false,
				},
			},
		}
	case "installing_runner":
		return map[string]any{
			"rbac": map[string]any{
				"create": true,
			},
			"runners": map[string]any{
				// 차트 0.72.0 은 runners.privileged 를 읽지 않는다 — 설정은
				// runners.config 의 TOML 로 넣어야 config.toml 에 반영된다.
				// privileged 없이는 docker:dind 서비스가 기동하지 못해
				// 파이프라인의 이미지 빌드 단계가 통째로 실패한다.
				"config": `[[runners]]
  [runners.kubernetes]
    namespace = "{{.Release.Namespace}}"
    image = "alpine"
    privileged = true
`,
			},
		}
	case "installing_prometheus":
		return map[string]any{
			"prometheus": map[string]any{
				"ingress": map[string]any{
					"enabled": false,
				},
			},
			"alertmanager": map[string]any{
				"ingress": map[string]any{
					"enabled": false,
				},
			},
			"grafana": map[string]any{
				"enabled": false,
			},
		}
	case "installing_grafana":
		return map[string]any{
			"adminUser": "admin",
			"ingress": map[string]any{
				"enabled": false,
			},
		}
	case "installing_logging":
		return map[string]any{
			"rbac": map[string]any{
				"pspEnabled": false,
			},
			"loki": map[string]any{
				"enabled": true,
			},
			"promtail": map[string]any{
				"enabled": true,
			},
			"grafana": map[string]any{
				"enabled": false,
			},
		}
	case "installing_logging_opensearch":
		return map[string]any{
			"singleNode": true,
			"protocol":   "http",
			"securityConfig": map[string]any{
				"enabled": false,
			},
			"config": map[string]any{
				"opensearch.yml": "cluster.name: opensearch-cluster\nnetwork.host: 0.0.0.0\nplugins.security.disabled: true\n",
			},
			"extraEnvs": []map[string]any{
				{
					"name":  "OPENSEARCH_INITIAL_ADMIN_PASSWORD",
					"value": "NullusAdmin123!",
				},
			},
		}
	case "installing_logging_elasticsearch":
		return map[string]any{
			"replicas": 1,
		}
	case "installing_opentelemetry":
		return map[string]any{
			"mode": "deployment",
		}
	case stepInstallingOTelCollector:
		// 설정과 무관한 기본 골격만 낸다. 어디로 내보낼지는 스택 설정을 아는
		// valuesForStep 이 이 위에 덮는다.
		return otelCollectorValues(nil)
	case "installing_tempo":
		return map[string]any{}
	case "installing_jaeger":
		return map[string]any{}
	default:
		return map[string]any{}
	}
}

func randomArgoCDServerSecretKey() string {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "nullus-argocd-server-secretkey"
	}
	return base64.StdEncoding.EncodeToString(key)
}

// jenkinsGiteaCredentialJCasC 는 Gitea 접속 credential 을 선언하는 JCasC 조각이다.
//
// 값은 ESO 가 동기화한 Secret 에서 보간된다 — OpenBao 가 여전히 단일 출처이고
// Jenkins Credentials 는 파생 사본일 뿐이다. 차트는 마운트된 Secret 을
// {name}-{keyName} 으로 노출한다.
func jenkinsGiteaCredentialJCasC() string {
	return `credentials:
  system:
    domainCredentials:
      - credentials:
          - usernamePassword:
              scope: GLOBAL
              id: "` + JenkinsGiteaCredentialID + `"
              username: "${` + domain.GiteaAdminSecret + `-` + domain.GiteaAdminUserKey + `}"
              password: "${` + domain.GiteaAdminSecret + `-` + domain.GiteaAdminPasswordKey + `}"
              description: "Nullus 가 만든 Gitea 접속 자격증명"
`
}

// jenkinsGiteaServerValues 는 gitea 플러그인에 서버를 등록한다.
//
// 주소가 네임스페이스에 달려 있어 DefaultValues 에 넣을 수 없다.
// 등록하지 않으면 job 의 SCM 소스가 가리키는 서버를 플러그인이 모르는 서버로
// 보고 스캔을 거부한다.
func jenkinsGiteaServerValues(namespace string) map[string]any {
	if strings.TrimSpace(namespace) == "" {
		namespace = defaultStackNamespace
	}
	serverURL := fmt.Sprintf("http://%s.%s.svc:%d",
		domain.GiteaHTTPServiceName, namespace, domain.GiteaServicePort)

	script := `unclassified:
  giteaServers:
    servers:
      - displayName: "nullus-gitea"
        serverUrl: "` + serverURL + `"
        manageHooks: false
        credentialsId: "` + JenkinsGiteaCredentialID + `"
`
	return map[string]any{
		"controller": map[string]any{
			"JCasC": map[string]any{
				"configScripts": map[string]any{
					"nullus-gitea-server": script,
				},
			},
		},
	}
}
