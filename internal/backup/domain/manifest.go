package domain

import "time"

// 매니페스트. 설계 §4.4 (nullus-plan#75)
//
// 이것만 암호화하지 않는다 — 키를 잃은 상황에서도 "무엇이 들어 있고 어떤
// 키가 필요한가" 는 읽을 수 있어야 한다. 그 대가로 **비밀값을 한 조각도
// 담지 않는 것**이 불변식이며, manifest_test.go 가 이를 고정한다.

type VolumeSpec struct {
	Name           string `json:"name"`
	SizeBytes      int64  `json:"size_bytes"`
	StorageClass   string `json:"storage_class,omitempty"`
	ChecksumSHA256 string `json:"checksum_sha256,omitempty"`
}

type WorkloadSpec struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	// OriginalReplicas 는 복원의 필수 입력이다 (설계 §3.4 3단계).
	OriginalReplicas int32 `json:"original_replicas"`
}

type QuiesceWindow struct {
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
}

// Duration 은 사용자가 실제로 감수한 다운타임이다. 정지 창 정책을 조정할
// 유일한 근거이므로 이력에 남긴다.
func (w *QuiesceWindow) Duration() time.Duration {
	if w == nil {
		return 0
	}
	return w.EndedAt.Sub(w.StartedAt)
}

type ComponentEntry struct {
	Component      Component `json:"component"`
	ResourceName   string    `json:"resource_name,omitempty"`
	Location       string    `json:"location"`
	SizeBytes      int64     `json:"size_bytes"`
	ChecksumSHA256 string    `json:"checksum_sha256"`
}

// EncryptionInfo 는 "어떤 키로 잠갔는지" 만 담는다. 키 자체는 절대 담지 않는다.
type EncryptionInfo struct {
	KeyID     string `json:"key_id"`
	Algorithm string `json:"algorithm"`
}

const EncryptionAlgorithm = "AES-256-GCM"

type Manifest struct {
	BackupRunID     string  `json:"backup_run_id"`
	OrgID           string  `json:"org_id"`
	StackID         string  `json:"stack_id,omitempty"`
	Mode            Mode    `json:"mode"`
	Trigger         Trigger `json:"trigger"`
	SchemaVersion   int     `json:"schema_version"`
	PlatformVersion string  `json:"platform_version,omitempty"`

	// pg_dump 는 서버 버전보다 낮으면 실패한다 (설계 §3.1 버전 함정).
	PGServerVersion string `json:"pg_server_version,omitempty"`
	PGDumpVersion   string `json:"pg_dump_version,omitempty"`

	Workloads []WorkloadSpec `json:"workloads,omitempty"`
	Volumes   []VolumeSpec   `json:"volumes,omitempty"`

	QuiesceWindow *QuiesceWindow `json:"quiesce_window,omitempty"`

	Components []ComponentEntry `json:"components"`
	// 복구 시 참조 정합성 검사의 기대값 (설계 §6.4).
	OpenBaoKVPathCount int `json:"openbao_kv_path_count,omitempty"`

	Encryption EncryptionInfo `json:"encryption"`
	CreatedAt  time.Time      `json:"created_at"`
}

type ManifestInput struct {
	PlatformVersion    string
	PGServerVersion    string
	PGDumpVersion      string
	EncryptionKeyID    string
	Plan               QuiescePlan
	Volumes            []VolumeSpec
	Artifacts          []Artifact
	OpenBaoKVPathCount int
}

func BuildManifest(run *BackupRun, in ManifestInput) Manifest {
	m := Manifest{
		BackupRunID:        run.ID,
		OrgID:              run.OrgID,
		StackID:            run.StackID,
		Mode:               run.Mode,
		Trigger:            run.Trigger,
		SchemaVersion:      run.SchemaVersion,
		PlatformVersion:    in.PlatformVersion,
		PGServerVersion:    in.PGServerVersion,
		PGDumpVersion:      in.PGDumpVersion,
		Volumes:            in.Volumes,
		OpenBaoKVPathCount: in.OpenBaoKVPathCount,
		Encryption:         EncryptionInfo{KeyID: in.EncryptionKeyID, Algorithm: EncryptionAlgorithm},
		CreatedAt:          run.CreatedAt,
	}

	for _, t := range in.Plan.Targets {
		m.Workloads = append(m.Workloads, WorkloadSpec{
			Kind:             t.Kind,
			Namespace:        t.Namespace,
			Name:             t.Name,
			OriginalReplicas: t.OriginalReplicas,
		})
	}

	if run.QuiesceStartedAt != nil && run.QuiesceEndedAt != nil {
		m.QuiesceWindow = &QuiesceWindow{StartedAt: *run.QuiesceStartedAt, EndedAt: *run.QuiesceEndedAt}
	}

	m.Components = make([]ComponentEntry, 0, len(in.Artifacts))
	for _, a := range in.Artifacts {
		m.Components = append(m.Components, ComponentEntry{
			Component:      a.Component,
			ResourceName:   a.ResourceName,
			Location:       a.Location,
			SizeBytes:      a.SizeBytes,
			ChecksumSHA256: a.ChecksumSHA256,
		})
	}
	return m
}
