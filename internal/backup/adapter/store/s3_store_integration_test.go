//go:build integration

// 실제 MinIO 로 산출물 저장소를 검증한다.
//
// 목적지는 백업 설계에서 유일하게 "밖" 에 있는 것이라(§4.2), 여기가 조용히
// 깨지면 백업본이 없는데 있다고 믿는 상태가 된다 — 가장 비싼 착각이다.
//
// 이미지는 에어갭 번들에 있는 것을 쓴다(airgap/images/images.txt).
// 실행: go test -tags integration ./internal/backup/adapter/store/
package store

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	testImage  = "quay.io/minio/minio:RELEASE.2024-12-18T13-15-44Z"
	testUser   = "nullus-admin"
	testSecret = "nullus-minio-secret"
	testBucket = "nullus-backup"
)

func TestS3Store_Preflight_정상(t *testing.T) {
	s, _ := setupMinIO(t, true)
	require.NoError(t, s.Preflight(context.Background(), 0))
}

// 정지 창에 들어가기 전에 막아야 하는 것들 (§9 F7b·F8).
func TestS3Store_Preflight_버킷이_없으면_실패한다(t *testing.T) {
	s, _ := setupMinIO(t, false) // 버킷을 만들지 않는다

	err := s.Preflight(context.Background(), 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), testBucket)
}

func TestS3Store_Preflight_자격증명이_틀리면_실패한다(t *testing.T) {
	_, cfg := setupMinIO(t, true)
	cfg.SecretKey = "틀린-비밀번호"
	bad, err := New(cfg)
	require.NoError(t, err)

	require.Error(t, bad.Preflight(context.Background(), 0))
}

func TestS3Store_Preflight_는_흔적을_남기지_않는다(t *testing.T) {
	// 쓰기 권한 확인용 probe 오브젝트가 남으면 보존 정책이 헷갈린다.
	s, cfg := setupMinIO(t, true)
	require.NoError(t, s.Preflight(context.Background(), 0))

	cl := rawClient(t, cfg)
	var n int
	for range cl.ListObjects(context.Background(), testBucket, minio.ListObjectsOptions{Recursive: true}) {
		n++
	}
	assert.Zero(t, n, "probe 오브젝트가 지워져야 한다")
}

func TestS3Store_Put_Get_왕복과_체크섬(t *testing.T) {
	s, _ := setupMinIO(t, true)
	ctx := context.Background()

	data := make([]byte, 3*1024*1024) // 멀티파트 경계를 넘겨 본다
	_, _ = rand.Read(data)
	want := sha256.Sum256(data)

	res, err := s.Put(ctx, "backup-r1/platform-db.dump", bytes.NewReader(data))
	require.NoError(t, err)
	assert.Equal(t, int64(len(data)), res.Bytes)
	assert.Equal(t, hex.EncodeToString(want[:]), res.ChecksumSHA256,
		"업로드하면서 계산한 해시가 내용과 맞아야 한다")
	assert.Equal(t, "s3://"+testBucket+"/backup-r1/platform-db.dump", res.Location)

	rc, err := s.Get(ctx, "backup-r1/platform-db.dump")
	require.NoError(t, err)
	defer func() { _ = rc.Close() }()
	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.True(t, bytes.Equal(data, got))
}

// ETag 를 쓰지 않고 sha256 을 메타데이터에 남기는 이유를 고정한다 —
// 멀티파트에서 ETag 는 내용 해시가 아니다.
func TestS3Store_Stat_은_저장된_sha256_을_돌려준다(t *testing.T) {
	s, _ := setupMinIO(t, true)
	ctx := context.Background()

	data := make([]byte, 6*1024*1024) // 확실히 멀티파트
	_, _ = rand.Read(data)

	put, err := s.Put(ctx, "backup-r1/big.tar", bytes.NewReader(data))
	require.NoError(t, err)

	size, sum, err := s.Stat(ctx, "backup-r1/big.tar")
	require.NoError(t, err)
	assert.Equal(t, int64(len(data)), size)
	assert.Equal(t, put.ChecksumSHA256, sum, "Stat 이 Put 과 같은 해시를 줘야 검증이 성립한다")
}

func TestS3Store_Get_없는_오브젝트(t *testing.T) {
	s, _ := setupMinIO(t, true)
	_, err := s.Get(context.Background(), "backup-없음/x")
	require.Error(t, err)
}

func TestS3Store_Delete_는_접두사_아래를_전부_지운다(t *testing.T) {
	s, cfg := setupMinIO(t, true)
	ctx := context.Background()

	for _, k := range []string{
		"backup-r1/platform-db.dump",
		"backup-r1/volumes/data-gitlab.tar",
		"backup-r2/platform-db.dump", // 다른 실행 — 남아야 한다
	} {
		_, err := s.Put(ctx, k, bytes.NewReader([]byte("x")))
		require.NoError(t, err)
	}

	require.NoError(t, s.Delete(ctx, "backup-r1/"))

	cl := rawClient(t, cfg)
	var left []string
	for o := range cl.ListObjects(ctx, testBucket, minio.ListObjectsOptions{Recursive: true}) {
		left = append(left, o.Key)
	}
	assert.Equal(t, []string{"backup-r2/platform-db.dump"}, left,
		"다른 백업 실행의 산출물까지 지우면 안 된다")
}

func TestS3Store_Prefix_로_환경을_나눈다(t *testing.T) {
	// 여러 환경이 한 버킷을 나눠 쓸 때 서로를 덮지 않아야 한다.
	_, cfg := setupMinIO(t, true)
	cfg.Prefix = "prod"
	s, err := New(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	res, err := s.Put(ctx, "backup-r1/x", bytes.NewReader([]byte("hello")))
	require.NoError(t, err)
	assert.Contains(t, res.Location, "prod/backup-r1/x")

	rc, err := s.Get(ctx, "backup-r1/x")
	require.NoError(t, err)
	defer func() { _ = rc.Close() }()
	got, _ := io.ReadAll(rc)
	assert.Equal(t, "hello", string(got))
}

func TestNew_설정_검증(t *testing.T) {
	_, err := New(Config{Bucket: "b"})
	require.Error(t, err, "엔드포인트가 없으면 거부한다")
	_, err = New(Config{Endpoint: "e"})
	require.Error(t, err, "버킷이 없으면 거부한다")
}

func TestStripScheme(t *testing.T) {
	assert.Equal(t, "minio.internal:9000", stripScheme("https://minio.internal:9000"))
	assert.Equal(t, "minio.internal:9000", stripScheme("http://minio.internal:9000/"))
	assert.Equal(t, "minio.internal:9000", stripScheme("minio.internal:9000"))
}

// ── 컨테이너 준비 ────────────────────────────────────────────────────────

func setupMinIO(t *testing.T, createBucket bool) (*S3Store, Config) {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        testImage,
		ExposedPorts: []string{"9000/tcp"},
		Cmd:          []string{"server", "/data"},
		Env: map[string]string{
			"MINIO_ROOT_USER":     testUser,
			"MINIO_ROOT_PASSWORD": testSecret,
		},
		WaitingFor: wait.ForHTTP("/minio/health/live").WithPort("9000/tcp").
			WithStartupTimeout(2 * time.Minute),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req, Started: true,
	})
	if err != nil {
		t.Fatalf("minio 컨테이너 기동: %v", err)
	}
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })

	host, err := c.Host(ctx)
	require.NoError(t, err)
	port, err := c.MappedPort(ctx, "9000")
	require.NoError(t, err)

	cfg := Config{
		Endpoint:  fmt.Sprintf("%s:%s", host, port.Port()),
		AccessKey: testUser,
		SecretKey: testSecret,
		Bucket:    testBucket,
		Region:    "us-east-1",
		UseSSL:    false,
	}

	if createBucket {
		cl := rawClient(t, cfg)
		require.NoError(t, cl.MakeBucket(ctx, testBucket, minio.MakeBucketOptions{Region: cfg.Region}))
	}

	s, err := New(cfg)
	require.NoError(t, err)
	return s, cfg
}

func rawClient(t *testing.T, cfg Config) *minio.Client {
	t.Helper()
	cl, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: false,
		Region: cfg.Region,
	})
	require.NoError(t, err)
	return cl
}
