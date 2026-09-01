package domain

import "time"

// Artifact 는 산출물 1건이다. 설계 §7.1
//
// 값 자체는 담지 않는다 — Location 은 오브젝트 스토리지 경로이고,
// EncryptionKeyID 는 "어떤 키로 잠갔는지" 만 가리킨다 (설계 §5.1).
type Artifact struct {
	ID              string
	BackupRunID     string
	Component       Component
	ResourceName    string // Component=volume 일 때 PVC 이름
	Location        string
	SizeBytes       int64
	ChecksumSHA256  string
	EncryptionKeyID string
	CreatedAt       time.Time
}
