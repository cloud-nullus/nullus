package domain

import (
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleRun() *BackupRun {
	run := NewBackupRun("org-1", ModeFull, TriggerManual, ModeComponents(ModeFull))
	run.ID = "run-1"
	run.StackID = "stack-1"
	run.SchemaVersion = 74
	_ = run.Start()
	run.BeginQuiesce()
	run.EndQuiesce()
	return run
}

func TestBuildManifest_복구에_필요한_값을_담는다(t *testing.T) {
	run := sampleRun()
	m := BuildManifest(run, ManifestInput{
		PlatformVersion: "0.5.0",
		PGServerVersion: "17.5",
		PGDumpVersion:   "17.5",
		EncryptionKeyID: "key-1",
		Plan: NewQuiescePlan([]Workload{
			{Kind: "Deployment", Namespace: "nullus", Name: "gitlab", Replicas: 3},
		}),
		Volumes: []VolumeSpec{{Name: "data-gitlab", SizeBytes: 100, StorageClass: "local-path"}},
		Artifacts: []Artifact{
			{Component: ComponentPlatformDB, Location: "s3://b/platform.dump", ChecksumSHA256: "abc", SizeBytes: 10},
		},
		OpenBaoKVPathCount: 12,
	})

	assert.Equal(t, 74, m.SchemaVersion)
	assert.Equal(t, "0.5.0", m.PlatformVersion)
	assert.Equal(t, "17.5", m.PGServerVersion)

	// 재개의 유일한 근거다. 빠지면 복구 시 몇으로 되돌릴지 알 수 없다.
	require.Len(t, m.Workloads, 1)
	assert.Equal(t, int32(3), m.Workloads[0].OriginalReplicas)

	require.Len(t, m.Volumes, 1)
	assert.Equal(t, "local-path", m.Volumes[0].StorageClass)

	require.NotNil(t, m.QuiesceWindow)
	assert.Equal(t, 12, m.OpenBaoKVPathCount)
	assert.Equal(t, "key-1", m.Encryption.KeyID)
	assert.Equal(t, "AES-256-GCM", m.Encryption.Algorithm)
}

// 설계 §4.4 — 매니페스트는 암호화하지 않는다. 키를 잃은 상황에서도 "무엇이
// 들어 있고 어떤 키가 필요한가" 는 읽을 수 있어야 하기 때문이다. 그래서
// 비밀값이 한 조각도 들어가지 않는 것이 불변식이다.
func TestBuildManifest_비밀값을_담지_않는다(t *testing.T) {
	run := sampleRun()
	m := BuildManifest(run, ManifestInput{
		PlatformVersion: "0.5.0",
		EncryptionKeyID: "key-1",
		Artifacts: []Artifact{
			{Component: ComponentPlatformDB, Location: "s3://b/platform.dump", ChecksumSHA256: "abc"},
		},
	})

	raw, err := json.Marshal(m)
	require.NoError(t, err)

	forbidden := regexp.MustCompile(`(?i)"[^"]*(password|secret|token|access_key|private_key|unseal|credential)[^"]*"\s*:`)
	assert.False(t, forbidden.Match(raw),
		"매니페스트에 비밀값으로 보이는 키가 있다: %s", string(raw))
}

func TestManifest_QuiesceDuration(t *testing.T) {
	start := time.Date(2026, 9, 1, 3, 0, 0, 0, time.UTC)
	end := start.Add(17 * time.Minute)
	w := &QuiesceWindow{StartedAt: start, EndedAt: end}
	assert.Equal(t, 17*time.Minute, w.Duration())
}

func TestBuildManifest_정지창이_없으면_nil(t *testing.T) {
	run := NewBackupRun("org-1", ModePlatformOnly, TriggerManual, ModeComponents(ModePlatformOnly))
	_ = run.Start()
	m := BuildManifest(run, ManifestInput{})
	assert.Nil(t, m.QuiesceWindow, "무중단 백업은 정지 창이 없다")
}

func TestBuildManifest_차트_버전을_담는다(t *testing.T) {
	// 복구가 다른 차트 버전으로 재설치하면 기동 시 스키마 마이그레이션이 돌아
	// **데이터가 백업 시점의 모습이 아니게 된다.** 되돌릴 수 없는 변형이므로,
	// "어느 버전이 이 데이터를 만들었는가" 를 산출물과 함께 남겨야 한다.
	//
	// 매니페스트는 암호화하지 않는 유일한 조각이라, 키를 잃은 상황에서도
	// 이 값을 읽어 같은 버전을 고를 수 있다.
	m := BuildManifest(sampleRun(), ManifestInput{
		HelmReleases: []HelmReleaseSpec{
			{Name: "gitea", Chart: "gitea", Version: "10.4.1", AppVersion: "1.22.3", Revision: 2},
			{Name: "harbor", Chart: "harbor", Version: "1.15.1", AppVersion: "2.11.1", Revision: 1},
		},
	})

	require.Len(t, m.HelmReleases, 2)
	assert.Equal(t, "gitea", m.HelmReleases[0].Name)
	assert.Equal(t, "10.4.1", m.HelmReleases[0].Version)
	assert.Equal(t, 2, m.HelmReleases[0].Revision)

	raw, err := json.Marshal(m)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "helm_releases")
}

func TestBuildManifest_차트가_없으면_필드를_비운다(t *testing.T) {
	// 스택 없는 플랫폼 백업에는 릴리스가 없다. 빈 배열로 남기면 "조회했는데
	// 없었다" 와 "조회하지 않았다" 가 구분되지 않으므로 생략한다.
	raw, err := json.Marshal(BuildManifest(sampleRun(), ManifestInput{}))
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "helm_releases")
}

func TestCompareHelmReleases_버전이_다르면_알린다(t *testing.T) {
	// 대상 네임스페이스에 이미 다른 버전이 설치돼 있으면, 복구는 그 위에
	// 옛 데이터를 얹게 된다. 도구가 기동하며 스키마를 올리면 백업 시점으로
	// 되돌아가지 않는다 — 되돌릴 수 없는 변형이라 미리 말해야 한다.
	drift := CompareHelmReleases(
		[]HelmReleaseSpec{
			{Name: "gitea", Version: "10.4.1"},
			{Name: "harbor", Version: "1.15.1"},
		},
		[]HelmReleaseSpec{
			{Name: "gitea", Version: "10.6.0"}, // 더 새 버전이 깔려 있다
			{Name: "harbor", Version: "1.15.1"},
		})
	require.Len(t, drift, 1)
	assert.Equal(t, "gitea", drift[0].Release)
	assert.Equal(t, "10.4.1", drift[0].BackedUp)
	assert.Equal(t, "10.6.0", drift[0].Current)
}

func TestCompareHelmReleases_같으면_비어있다(t *testing.T) {
	assert.Empty(t, CompareHelmReleases(
		[]HelmReleaseSpec{{Name: "gitea", Version: "10.4.1"}},
		[]HelmReleaseSpec{{Name: "gitea", Version: "10.4.1"}}))
}

func TestCompareHelmReleases_대상에_없으면_문제_아니다(t *testing.T) {
	// 빈 네임스페이스로 복구하는 것이 정상 경로다. 설치될 것이 없으니
	// 어긋날 것도 없다.
	assert.Empty(t, CompareHelmReleases(
		[]HelmReleaseSpec{{Name: "gitea", Version: "10.4.1"}}, nil))
}

func TestCompareHelmReleases_백업에_기록이_없으면_비교하지_않는다(t *testing.T) {
	// 옛 백업에는 이 정보가 없다. 없는 것을 "다르다" 로 읽으면 멀쩡한 복구가
	// 경고투성이가 된다.
	assert.Empty(t, CompareHelmReleases(nil,
		[]HelmReleaseSpec{{Name: "gitea", Version: "10.6.0"}}))
}
