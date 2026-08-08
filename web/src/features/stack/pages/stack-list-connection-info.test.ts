import { describe, expect, it } from "vitest";
import {
	buildConnectionInfoText,
	buildOssLoginHint,
	findToolCredential,
	toConnectionInfoView,
	type LaunchTool,
} from "./stack-list-page";
import type { StackConnectionInfoResponse } from "../api/stack-api-types";

const serverConnection: StackConnectionInfoResponse = {
	stack_id: "stk_1",
	namespace: "devsecops",
	access_domain: "nullus-devsecops-stack.internal",
	database: {
		mode: "existing-connect",
		engine: "postgres",
		endpoint: "db.prod.svc:5432",
		resource_name: "prod_db",
		auth_id: "prod_user",
		secret_ref: "prod-db-secret",
		secret_key: "password",
	},
	object_storage: {
		mode: "create",
		engine: "minio",
		endpoint: "http://nullus-minio:9000",
		resource_name: "gitlab-artifacts",
		auth_id: "nullus-admin",
		secret_ref: "nullus-minio-credentials",
		secret_key: "rootPassword",
	},
	tools: [
		{
			name: "Argo CD",
			username: "admin",
			secret_ref: "argocd-initial-admin-secret",
			secret_key: "password",
		},
		{ name: "OpenSearch", username: "admin", note: "차트 기본값을 확인하세요." },
	],
};

describe("stack connection info helpers", () => {
	// 사용자가 지정한 외부 스토리지든 설치가 만든 것이든, 화면은 서버가 준
	// 값을 그대로 보여준다.
	it("shows the storage values the server returned", () => {
		const info = toConnectionInfoView(serverConnection);
		expect(info.database.mode).toBe("existing-connect");
		expect(info.database.endpoint).toBe("db.prod.svc:5432");
		expect(info.database.authId).toBe("prod_user");
		expect(info.objectStorage.endpoint).toBe("http://nullus-minio:9000");
		expect(info.objectStorage.authPasswordKey).toBe("rootPassword");
	});

	it("builds oss login hints and combined connection text", () => {
		const info = toConnectionInfoView(serverConnection);
		expect(
			buildOssLoginHint(findToolCredential(serverConnection.tools, "argocd"), info.namespace),
		).toContain("admin");
		expect(
			buildOssLoginHint(findToolCredential(serverConnection.tools, "opensearch"), info.namespace),
		).toContain("차트 기본값");

		const tools: LaunchTool[] = [
			{ name: "ArgoCD", version: "v2", url: "http://argocd.nullus-devsecops-stack.internal", logo: "" },
			{ name: "OpenSearch", version: "v2", url: "http://opensearch.nullus-devsecops-stack.internal", logo: "" },
		];

		const text = buildConnectionInfoText(
			"nullus-devsecops-stack",
			info,
			tools,
			serverConnection.tools,
		);
		expect(text).toContain("[OSS Login]");
		expect(text).toContain("ArgoCD");
		expect(text).toContain("secret argocd-initial-admin-secret");
		expect(text).toContain("[Database]");
		expect(text).toContain("[Object Storage]");
	});
});
