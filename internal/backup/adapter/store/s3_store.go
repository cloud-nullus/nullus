// Package store 는 백업 산출물 저장소 어댑터다.
//
// 설계: docs/11_기능설계/Nullus_백업복구_설계.md §4.2 (nullus-plan#75)
//
// 목적지는 **대상 클러스터 밖**의 S3 호환 오브젝트 스토리지다. 클러스터·
// 노드·스토리지가 통째로 사라져도 백업본은 남아야 하기 때문이다.
package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/cloud-nullus/draft/internal/backup/port"
)

// checksumMetaKey 는 sha256 을 오브젝트 사용자 메타데이터에 남길 때 쓰는 키다.
//
// ETag 를 쓰지 않는 이유: 멀티파트 업로드에서 ETag 는 내용 해시가 아니라
// 조각 해시의 해시라서, 같은 내용이라도 조각 크기에 따라 달라진다. 무결성
// 검증의 기준으로 쓸 수 없다.
const checksumMetaKey = "Nullus-Sha256"

type Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	Region    string
	UseSSL    bool
	// Prefix 는 버킷 안의 최상위 경로다. 여러 환경이 한 버킷을 나눠 쓸 때 쓴다.
	Prefix string
}

type S3Store struct {
	client *minio.Client
	cfg    Config
}

func New(cfg Config) (*S3Store, error) {
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return nil, errors.New("백업 목적지 엔드포인트가 설정되지 않았습니다")
	}
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, errors.New("백업 목적지 버킷이 설정되지 않았습니다")
	}
	cl, err := minio.New(stripScheme(cfg.Endpoint), &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("오브젝트 스토리지 클라이언트 생성: %w", err)
	}
	return &S3Store{client: cl, cfg: cfg}, nil
}

func stripScheme(ep string) string {
	ep = strings.TrimPrefix(ep, "https://")
	ep = strings.TrimPrefix(ep, "http://")
	return strings.TrimSuffix(ep, "/")
}

func (s *S3Store) key(k string) string {
	if s.cfg.Prefix == "" {
		return k
	}
	return strings.TrimSuffix(s.cfg.Prefix, "/") + "/" + k
}

// Preflight 는 정지 창에 들어가기 전에 목적지를 검사한다.
//
// 정지 창을 소비하고도 산출물을 못 만드는 것이 최악의 조합이므로 (§9 F7b·F8),
// 연결·인증·쓰기 권한을 **멈추기 전에** 확인한다.
func (s *S3Store) Preflight(ctx context.Context, requiredBytes int64) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	ok, err := s.client.BucketExists(ctx, s.cfg.Bucket)
	if err != nil {
		return fmt.Errorf("목적지에 연결할 수 없습니다 (%s): %w", s.cfg.Endpoint, err)
	}
	if !ok {
		return fmt.Errorf("버킷 %q 이 없습니다", s.cfg.Bucket)
	}

	// 쓰기 권한은 실제로 써 봐야 안다. 읽기만 되는 자격증명으로 정지 창에
	// 들어가면 그 시간을 통째로 버린다.
	probe := s.key(fmt.Sprintf(".nullus-preflight-%d", time.Now().UnixNano()))
	body := strings.NewReader("nullus backup preflight")
	if _, err := s.client.PutObject(ctx, s.cfg.Bucket, probe, body, body.Size(),
		minio.PutObjectOptions{ContentType: "text/plain"}); err != nil {
		return fmt.Errorf("목적지에 쓸 수 없습니다: %w", err)
	}
	if err := s.client.RemoveObject(ctx, s.cfg.Bucket, probe, minio.RemoveObjectOptions{}); err != nil {
		// 지우지 못한 것은 치명적이지 않다 — 쓰기는 됐으므로 진행한다.
		_ = err
	}
	return nil
}

// Put 은 스트리밍으로 올리면서 sha256 을 함께 계산한다.
//
// 크기를 미리 모르므로 -1 을 넘긴다. minio-go 가 멀티파트로 나눠 올린다.
func (s *S3Store) Put(ctx context.Context, key string, r io.Reader) (port.PutResult, error) {
	h := sha256.New()
	tee := io.TeeReader(r, h)
	full := s.key(key)

	info, err := s.client.PutObject(ctx, s.cfg.Bucket, full, tee, -1, minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	if err != nil {
		return port.PutResult{}, fmt.Errorf("산출물 업로드(%s): %w", key, err)
	}
	sum := hex.EncodeToString(h.Sum(nil))

	// 해시를 오브젝트에 붙여 둔다. 이후 verify 는 다시 내려받지 않고
	// 메타데이터만 읽어 대조한다.
	src := minio.CopySrcOptions{Bucket: s.cfg.Bucket, Object: full}
	dst := minio.CopyDestOptions{
		Bucket:          s.cfg.Bucket,
		Object:          full,
		UserMetadata:    map[string]string{checksumMetaKey: sum},
		ReplaceMetadata: true,
	}
	if _, err := s.client.CopyObject(ctx, dst, src); err != nil {
		// 메타데이터를 못 붙여도 업로드 자체는 성공했다. 해시는 DB 에도
		// 저장되므로 검증 경로가 완전히 끊기지는 않는다.
		_ = err
	}

	return port.PutResult{
		Bytes:          info.Size,
		ChecksumSHA256: sum,
		Location:       fmt.Sprintf("s3://%s/%s", s.cfg.Bucket, full),
	}, nil
}

func (s *S3Store) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.cfg.Bucket, s.key(key), minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("산출물 다운로드(%s): %w", key, err)
	}
	// GetObject 는 지연 평가라 여기서 존재를 확인한다.
	if _, err := obj.Stat(); err != nil {
		_ = obj.Close()
		return nil, fmt.Errorf("산출물을 찾을 수 없습니다(%s): %w", key, err)
	}
	return obj, nil
}

func (s *S3Store) Stat(ctx context.Context, key string) (int64, string, error) {
	info, err := s.client.StatObject(ctx, s.cfg.Bucket, s.key(key), minio.StatObjectOptions{})
	if err != nil {
		return 0, "", err
	}
	sum := info.UserMetadata[checksumMetaKey]
	if sum == "" {
		sum = info.UserMetadata[strings.ToLower(checksumMetaKey)]
	}
	return info.Size, sum, nil
}

// Delete 는 접두사 아래를 전부 지운다 (한 백업 실행의 산출물 묶음).
func (s *S3Store) Delete(ctx context.Context, prefix string) error {
	full := s.key(prefix)
	objects := s.client.ListObjects(ctx, s.cfg.Bucket, minio.ListObjectsOptions{
		Prefix: full, Recursive: true,
	})
	for err := range s.client.RemoveObjects(ctx, s.cfg.Bucket, objects, minio.RemoveObjectsOptions{}) {
		if err.Err != nil {
			return fmt.Errorf("산출물 삭제(%s): %w", err.ObjectName, err.Err)
		}
	}
	return nil
}

var _ port.ArtifactStore = (*S3Store)(nil)
