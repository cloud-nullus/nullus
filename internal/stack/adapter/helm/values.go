package helm

import (
	"crypto/rand"
	"encoding/base64"
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
			"auth": map[string]any{
				"username":         "gitlab",
				"password":         "nullus-gitlab-password", // #nosec G101 -- default Helm value, expected to be overridden by operator
				"database":         "gitlabhq_production",
				"postgresPassword": "nullus-postgres-admin", // #nosec G101 -- default Helm value, expected to be overridden by operator
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
			"mode":         "standalone",
			"rootUser":     "nullus-admin",
			"rootPassword": "nullus-minio-secret", // #nosec G101 -- default Helm value, expected to be overridden by operator
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
					"host":     "nullus-postgresql.nullus.svc.cluster.local",
					"port":     5432,
					"database": "gitlabhq_production",
					"username": "gitlab",
					"password": map[string]any{
						"useSecret": true,
						"secret":    "nullus-postgresql",
						"key":       "password",
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
				"privileged": true,
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
