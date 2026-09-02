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
