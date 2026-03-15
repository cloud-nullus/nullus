package helm

func DefaultValues(stepName string) map[string]any {
	switch stepName {
	case "installing_cert_manager":
		return map[string]any{
			"installCRDs": true,
		}
	case "installing_minio":
		return map[string]any{
			"mode": "standalone",
		}
	case "installing_gitlab":
		return map[string]any{
			"global": map[string]any{
				"edition": "ce",
			},
		}
	case "installing_argocd":
		return map[string]any{
			"crds": map[string]any{
				"install": true,
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
			"grafana": map[string]any{
				"enabled": false,
			},
		}
	case "installing_grafana":
		return map[string]any{
			"adminUser": "admin",
		}
	default:
		return map[string]any{}
	}
}
