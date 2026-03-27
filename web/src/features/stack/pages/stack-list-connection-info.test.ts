import { describe, expect, it } from "vitest";
import { buildConnectionInfoText, buildOssLoginHint, extractConnectionInfo, type LaunchTool } from "./stack-list-page";

describe("stack connection info helpers", () => {
	it("extracts connection info from snapshot storage config", () => {
		const snapshot = {
			config: {
				storage: {
					database: {
						mode: "existing",
						provider_or_engine: "postgres",
						endpoint: "db.prod.svc:5432",
						resource_name: "prod_db",
						auth_id: "prod_user",
						access_secret_ref: "prod-db-secret",
						auth_password_key: "password",
					},
					object_storage: {
						mode: "create",
						provider_or_engine: "minio",
						endpoint: "http://minio.nullus.svc:9000",
						resource_name: "artifacts",
						auth_id: "minio_access",
						access_secret_ref: "minio-secret",
						auth_password_key: "secretKey",
					},
				},
			},
		};

		const info = extractConnectionInfo(snapshot, "nullus", "nullus-devsecops-stack.internal");
		expect(info.database.endpoint).toBe("db.prod.svc:5432");
		expect(info.database.authId).toBe("prod_user");
		expect(info.objectStorage.endpoint).toBe("http://minio.nullus.svc:9000");
		expect(info.objectStorage.authPasswordKey).toBe("secretKey");
	});

	it("provides safe defaults when storage config is missing", () => {
		const info = extractConnectionInfo({}, "nullus", "nullus-devsecops-stack.internal");
		expect(info.database.endpoint).toBe("nullus-postgresql:5432");
		expect(info.objectStorage.endpoint).toBe("http://nullus-minio:9000");
	});

	it("builds oss login hints and combined connection text", () => {
		const info = extractConnectionInfo({}, "nullus", "nullus-devsecops-stack.internal");
		expect(buildOssLoginHint("argocd", info)).toContain("admin");
		expect(buildOssLoginHint("opensearch", info)).toContain("NullusAdmin123!");

		const tools: LaunchTool[] = [
			{ name: "ArgoCD", version: "v2", url: "http://argocd.nullus-devsecops-stack.internal", logo: "" },
			{ name: "OpenSearch", version: "v2", url: "http://opensearch.nullus-devsecops-stack.internal", logo: "" },
		];

		const text = buildConnectionInfoText("nullus-devsecops-stack", info, tools);
		expect(text).toContain("[OSS Login]");
		expect(text).toContain("ArgoCD");
		expect(text).toContain("[Database]");
		expect(text).toContain("[Object Storage]");
	});
});
