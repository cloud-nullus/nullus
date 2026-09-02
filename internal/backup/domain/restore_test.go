package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 설계 §6.2 — 스키마 버전 정합성 검사

func TestCheckSchemaVersion_같으면_그대로_복원(t *testing.T) {
	res := CheckSchemaVersion(74, SchemaState{Version: 74, Dirty: false})
	assert.True(t, res.Allowed)
	assert.False(t, res.MigrateAfterRestore)
}

func TestCheckSchemaVersion_백업이_낮으면_복원후_마이그레이션(t *testing.T) {
	res := CheckSchemaVersion(70, SchemaState{Version: 74})
	assert.True(t, res.Allowed)
	assert.True(t, res.MigrateAfterRestore, "복원 먼저, 마이그레이션 나중이다")
}

func TestCheckSchemaVersion_백업이_높으면_차단(t *testing.T) {
	// 구버전 코드가 신버전 스키마를 읽는 것은 정의되지 않은 동작이다.
	res := CheckSchemaVersion(80, SchemaState{Version: 74})
	assert.False(t, res.Allowed)
	assert.Contains(t, res.Reason, "80")
}

func TestCheckSchemaVersion_dirty_면_차단(t *testing.T) {
	// 마이그레이션이 중단된 상태의 백업본은 신뢰할 수 없다.
	res := CheckSchemaVersion(74, SchemaState{Version: 74, Dirty: true})
	assert.False(t, res.Allowed)
	assert.Contains(t, res.Reason, "dirty")
}

func TestCheckSchemaVersion_백업_버전이_없으면_차단(t *testing.T) {
	res := CheckSchemaVersion(0, SchemaState{Version: 74})
	assert.False(t, res.Allowed)
}

// 설계 §6.1 0단계 — 전제 확인. 키가 없는 채로 진행하면 "복구된 것처럼 보이는
// 망가진 상태" 가 된다.

func TestCheckPrerequisites_전부_있으면_통과(t *testing.T) {
	err := CheckPrerequisites(Prerequisites{
		EncryptionKeyPresent:    true,
		BackupSealKeyPresent:    true,
		DestinationCredsPresent: true,
		DestinationReachable:    true,
	})
	require.NoError(t, err)
}

func TestCheckPrerequisites_ENCRYPTION_KEY_없으면_중단(t *testing.T) {
	// 이것이 없으면 clusters.kubeconfig 를 못 푼다 — 화면은 정상인데
	// 어떤 클러스터도 조작할 수 없는 상태가 된다 (설계 §1.3, F1).
	err := CheckPrerequisites(Prerequisites{
		BackupSealKeyPresent:    true,
		DestinationCredsPresent: true,
		DestinationReachable:    true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ENCRYPTION_KEY")
}

func TestCheckPrerequisites_목적지_자격증명_없으면_중단(t *testing.T) {
	err := CheckPrerequisites(Prerequisites{
		EncryptionKeyPresent: true,
		BackupSealKeyPresent: true,
		DestinationReachable: true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "목적지")
}

func TestCheckPrerequisites_여러개_빠지면_전부_알려준다(t *testing.T) {
	// 하나씩 고치고 다시 시도하게 만들면 복구가 그만큼 늦어진다.
	err := CheckPrerequisites(Prerequisites{})
	require.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, "ENCRYPTION_KEY")
	assert.Contains(t, msg, "목적지")
}
