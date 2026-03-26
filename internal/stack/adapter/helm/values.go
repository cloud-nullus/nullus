package helm

func DefaultValues(stepName string) map[string]any {
	switch stepName {
	case "installing_cert_manager":
		return map[string]any{
			"installCRDs": true,
		}
	case "installing_minio":
		return map[string]any{
			"mode":         "standalone",
			"rootUser":     "nullus-admin",
			"rootPassword": "nullus-minio-secret",
			"ingress": map[string]any{
				"enabled": false,
			},
			"consoleIngress": map[string]any{
				"enabled": false,
			},
			"resources": map[string]any{
				"requests": map[string]any{
					"memory": "512Mi",
				},
			},
		}
	case "installing_gitlab":
		return map[string]any{
			"global": map[string]any{
				"edition": "ce",
				"hosts": map[string]any{
					"domain": "nullus.internal",
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
			"minio": map[string]any{
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
			"postgresql": map[string]any{
				"image": map[string]any{
					"repository": "bitnamilegacy/postgresql",
					"tag":        "16.6.0-debian-12-r2",
				},
				"metrics": map[string]any{
					"image": map[string]any{
						"repository": "bitnamilegacy/postgres-exporter",
						"tag":        "0.17.1-debian-12-r16",
					},
				},
			},
			"redis": map[string]any{
				"image": map[string]any{
					"repository": "bitnamilegacy/redis",
					"tag":        "7.4.2-debian-12-r0",
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
	default:
		return map[string]any{}
	}
}
