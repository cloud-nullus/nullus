package domain

import "testing"

func TestObjectStorageBackendFor_SelectsByCatalogName(t *testing.T) {
	cases := []struct {
		name string
		sel  ToolSelection
		want string
		ok   bool
	}{
		// 이름 없이 저장된 기존 스택은 MinIO 로 남아야 한다 — 재시도에서 다른
		// 구현체를 깔면 이미 쌓인 오브젝트를 잃는다.
		{"empty name keeps MinIO", ToolSelection{Enabled: true}, ObjectStorageToolMinIO, true},
		{"MinIO", ToolSelection{Name: "MinIO", Enabled: true}, ObjectStorageToolMinIO, true},
		{"MinIO with remark", ToolSelection{Name: "MinIO (SILO)", Enabled: true}, ObjectStorageToolMinIO, true},
		{"SILO fork", ToolSelection{Name: "silo", Enabled: true}, ObjectStorageToolMinIO, true},
		{"SeaweedFS", ToolSelection{Name: "SeaweedFS", Enabled: true}, ObjectStorageToolSeaweedFS, true},
		{"seaweed_fs", ToolSelection{Name: "seaweed_fs", Enabled: true}, ObjectStorageToolSeaweedFS, true},
		{"disabled", ToolSelection{Name: "SeaweedFS"}, "", false},
		// 이 슬롯은 오래 이름을 보지 않고 MinIO 를 세웠다. 알아보지 못하는 이름의
		// 설치를 바꾸면 이미 그렇게 서 있는 스택이 재시도에서 데이터를 잃는다.
		{"unrecognized name keeps legacy MinIO", ToolSelection{Name: "s3", Enabled: true}, ObjectStorageToolMinIO, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ObjectStorageBackendFor(tc.sel)
			if ok != tc.ok || got.Tool != tc.want {
				t.Fatalf("ObjectStorageBackendFor(%+v) = (%q, %v), want (%q, %v)", tc.sel, got.Tool, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestObjectStorageBackendNamed_OnlyRecognizesImplementations(t *testing.T) {
	for name, want := range map[string]string{
		"": ObjectStorageToolMinIO, "MinIO": ObjectStorageToolMinIO, "SeaweedFS": ObjectStorageToolSeaweedFS,
	} {
		if got, ok := ObjectStorageBackendNamed(name); !ok || got.Tool != want {
			t.Errorf("ObjectStorageBackendNamed(%q) = (%q, %v), want %q", name, got.Tool, ok, want)
		}
	}
	// 클러스터 밖 제공자는 구현체가 아니다 — 호환성 매트릭스 검사를 그대로 받아야 한다.
	for _, name := range []string{"s3", "gcs", "azure-blob", "Ceph"} {
		if got, ok := ObjectStorageBackendNamed(name); ok {
			t.Errorf("ObjectStorageBackendNamed(%q) = %q, want not found", name, got.Tool)
		}
	}
}

// 구현체마다 다른 것은 표 하나에 모았다. 빈 칸이 있으면 그 구현체를 고른 스택이
// 설치 도중 어딘가에서 이름 없는 Secret·Service 를 찾다가 멈춘다.
func TestObjectStorageBackends_AreFullyDescribedAndDistinct(t *testing.T) {
	seen := map[string]string{}
	for _, b := range ObjectStorageBackends() {
		fields := map[string]string{
			"Tool": b.Tool, "DisplayName": b.DisplayName, "Step": b.Step,
			"ReleaseName": b.ReleaseName, "ServiceName": b.ServiceName,
			"CredentialSecret": b.CredentialSecret, "AccessKeyKey": b.AccessKeyKey,
			"SecretKeyKey": b.SecretKeyKey, "VaultAccessKeyPath": b.VaultAccessKeyPath,
			"VaultSecretKeyPath": b.VaultSecretKeyPath,
		}
		for field, value := range fields {
			if value == "" {
				t.Errorf("%s: %s 가 비어 있다", b.Tool, field)
			}
		}
		if b.ServicePort <= 0 {
			t.Errorf("%s: ServicePort 가 없다", b.Tool)
		}
		for _, unique := range []string{b.Step, b.ReleaseName, b.CredentialSecret, b.VaultSecretKeyPath} {
			if owner, dup := seen[unique]; dup {
				t.Errorf("%q 를 %s 와 %s 가 함께 쓴다", unique, owner, b.Tool)
			}
			seen[unique] = b.Tool
		}
		if got, ok := ObjectStorageBackendForStep(b.Step); !ok || got.Tool != b.Tool {
			t.Errorf("ObjectStorageBackendForStep(%q) = (%q, %v)", b.Step, got.Tool, ok)
		}
		if !containsString(InstallStepOrder, b.Step) {
			t.Errorf("%s 단계가 InstallStepOrder 에 없다 — 진행률과 오케스트레이터 순서가 이 단계를 모른다", b.Step)
		}
	}
}

// 기존 스택의 금고와 Secret 을 그대로 읽어야 하므로 MinIO 쪽 이름은 표가 생기기
// 전과 같아야 한다. 바뀌면 재설치가 새 비밀번호를 만들어 기존 데이터에 못 붙는다.
func TestMinIOObjectStorage_KeepsLegacyNames(t *testing.T) {
	b := MinIOObjectStorage
	if b.Step != "installing_minio" || b.ReleaseName != "nullus-minio" ||
		b.CredentialSecret != "nullus-minio-credentials" ||
		b.AccessKeyKey != "rootUser" || b.SecretKeyKey != "rootPassword" ||
		b.VaultAccessKeyPath != "artifacts/minio/root-user" ||
		b.VaultSecretKeyPath != "artifacts/minio/root-password" {
		t.Fatalf("MinIO 이름이 바뀌었다: %+v", b)
	}
	if got := b.Endpoint("team-a"); got != "http://nullus-minio.team-a.svc.cluster.local:9000" {
		t.Fatalf("Endpoint = %q", got)
	}
}

func TestSeaweedFSObjectStorage_EndpointIsAllInOneS3Gateway(t *testing.T) {
	got := SeaweedFSObjectStorage.Endpoint("")
	want := "http://nullus-seaweedfs-all-in-one." + DefaultStackNamespace + ".svc.cluster.local:8333"
	if got != want {
		t.Fatalf("Endpoint = %q, want %q", got, want)
	}
}

func containsString(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

// 모니터링 OSS 목록은 고른 구현체의 파드를 센다. MinIO 접두사를 그대로 두면
// SeaweedFS 를 고른 스택이 설치는 정상인데 화면에서만 "0 파드" 로 죽어 보인다.
func TestInstalledToolWorkloads_ObjectStorageFollowsBackend(t *testing.T) {
	for _, tc := range []struct {
		name string
		sel  ToolSelection
		want []string
	}{
		{"SeaweedFS", ToolSelection{Name: "SeaweedFS", Enabled: true}, []string{SeaweedFSReleaseName}},
		{"MinIO", ToolSelection{Name: "MinIO", Enabled: true}, []string{MinIOServiceName, "minio"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			items := InstalledToolWorkloads(StackConfig{Artifacts: ArtifactsConfig{StorageBackend: tc.sel}})
			got, ok := workloadByKey(items, "storage_backend")
			if !ok {
				t.Fatal("storage_backend 워크로드가 없다")
			}
			if len(got.NamePrefixes) != len(tc.want) {
				t.Fatalf("NamePrefixes = %v, want %v", got.NamePrefixes, tc.want)
			}
			for i := range tc.want {
				if got.NamePrefixes[i] != tc.want[i] {
					t.Fatalf("NamePrefixes = %v, want %v", got.NamePrefixes, tc.want)
				}
			}
		})
	}
	if _, ok := workloadByKey(InstalledToolWorkloads(StackConfig{}), "storage_backend"); ok {
		t.Fatal("고르지 않은 오브젝트 스토리지가 목록에 있다")
	}
}

// 표시 이름은 오류 메시지·설치 로그에 그대로 나간다. minio 를 고르면 서는 것은 MinIO 포크
// SILO 라 화면의 마크(silo.svg)와 같은 이름을 쓴다. 그 이름이 다시 돌아와도 같은 구현체다.
func TestObjectStorageBackends_DisplayNameNamesWhatGetsInstalled(t *testing.T) {
	if got := MinIOObjectStorage.DisplayName; got != "MinIO (SILO)" {
		t.Errorf("MinIO DisplayName = %q, want %q", got, "MinIO (SILO)")
	}
	for _, b := range ObjectStorageBackends() {
		got, ok := ObjectStorageBackendNamed(b.DisplayName)
		if !ok || got.Tool != b.Tool {
			t.Errorf("ObjectStorageBackendNamed(%q) = %q, %v — want %q", b.DisplayName, got.Tool, ok, b.Tool)
		}
	}
}
