// Package backup 은 백업/복구 컨텍스트의 조립 지점이다.
//
// 설계: docs/11_기능설계/Nullus_백업복구_설계.md (nullus-plan#75)
//
// 조립을 모듈 안에 두는 이유: 백업은 스택별 클러스터 어댑터를 요청 시점에
// 만들어야 해서 배선이 단순하지 않다. 그 복잡도를 cmd/api/main.go 에 흘리면
// main 이 이 모듈의 내부 사정을 알게 된다.
package backup

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/backup/adapter/bridge"
	backupkube "github.com/cloud-nullus/draft/internal/backup/adapter/kube"
	"github.com/cloud-nullus/draft/internal/backup/adapter/notify"
	backupopenbao "github.com/cloud-nullus/draft/internal/backup/adapter/openbao"
	backuppostgres "github.com/cloud-nullus/draft/internal/backup/adapter/postgres"
	backuprepo "github.com/cloud-nullus/draft/internal/backup/adapter/repository"
	"github.com/cloud-nullus/draft/internal/backup/adapter/sealer"
	"github.com/cloud-nullus/draft/internal/backup/adapter/store"
	"github.com/cloud-nullus/draft/internal/backup/domain"
	"github.com/cloud-nullus/draft/internal/backup/port"
	"github.com/cloud-nullus/draft/internal/backup/usecase"
	"github.com/cloud-nullus/draft/internal/shared/config"
	"github.com/cloud-nullus/draft/internal/shared/secrets"
)

// KubeconfigProvider 는 클러스터 ID 로 kubeconfig 를 준다 (stack 모듈 제공).
type KubeconfigProvider interface {
	GetKubeconfig(ctx context.Context, clusterID string) ([]byte, error)
}

// Pausable 은 백업 정지 창 동안 멈출 수 있는 것이다 (토큰 회전 스케줄러).
type Pausable interface {
	Pause()
	Resume()
}

type Deps struct {
	Pool            *pgxpool.Pool
	Config          config.BackupConfig
	PlatformDB      config.DatabaseConfig
	SecretRouter    *secrets.Router
	Kubeconfig      KubeconfigProvider
	TokenSources    bridge.AdminTokenSourceRepo
	RotationPausing Pausable
	// EncryptionKey 는 clusters.kubeconfig 복호화 키다 (ENCRYPTION_KEY).
	// 백업이 쓰지는 않지만, 복구 전제 조건 검사에 필요하다 — 이 키가 없으면
	// 복구해도 어떤 클러스터에도 접근할 수 없다 (§1.3).
	EncryptionKey   []byte
	PlatformVersion string
	Logger          *slog.Logger
}

// Module 은 조립된 백업 모듈이다.
type Module struct {
	Repo      *backuprepo.PostgresBackupRepository
	Verify    *usecase.VerifyUseCase
	Retention *usecase.RetentionUseCase

	deps     Deps
	store    port.ArtifactStore
	sealer   port.Sealer
	dumper   port.DBDumper
	kv       port.KVExporter
	targets  usecase.DBTargets
	tokens   port.TokenSourceLister
	notifier port.Notifier

	mu    sync.Mutex
	cache map[string]*backupkube.Adapters
}

// New 는 백업 모듈을 조립한다.
//
// 목적지나 키가 없으면 (nil, nil) 을 돌려준다 — 백업이 꺼진 것은 오류가
// 아니다. 다만 그 사실이 로그에 남아야 한다. "백업이 돌고 있다" 는 착각이
// 가장 비싸다 (§9 F10).
func New(d Deps) (*Module, error) {
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	if !d.Config.Enabled {
		d.Logger.Info("백업 기능이 꺼져 있습니다 (backup.enabled=false)")
		return nil, nil
	}
	if strings.TrimSpace(d.Config.Destination.Endpoint) == "" {
		return nil, fmt.Errorf("backup.enabled=true 인데 목적지가 설정되지 않았습니다. " +
			"백업본은 대상 클러스터 밖에 두어야 합니다")
	}
	key := []byte(d.Config.SealKey)
	if len(key) != 32 {
		return nil, fmt.Errorf("backup.seal_key 는 정확히 32바이트여야 합니다 (현재 %d)", len(key))
	}
	// 키를 돌려쓰면 하나를 잃을 때 둘 다 잃는다 (§5.2).
	//
	// ENCRYPTION_KEY 와 같은 값이 특히 위험하다 — 그 키는 clusters.kubeconfig
	// 를 여는 키이고, 백업본을 여는 키이기도 해지면 백업본 하나가 새는 순간
	// 등록된 모든 클러스터가 함께 열린다.
	if len(d.EncryptionKey) > 0 && d.Config.SealKey == string(d.EncryptionKey) {
		return nil, fmt.Errorf("backup.seal_key 가 ENCRYPTION_KEY 와 같습니다. " +
			"백업본이 새면 등록된 모든 클러스터가 함께 열립니다 — 서로 다른 키를 쓰세요")
	}
	if d.Config.SealKey == d.PlatformDB.Password {
		return nil, fmt.Errorf("backup.seal_key 가 DB 비밀번호와 같습니다. 서로 다른 키를 쓰세요")
	}

	// Keycloak DB 를 빠뜨리면 복구해도 아무도 로그인할 수 없다(§1.2).
	// 배포 경로에 따라 위치가 달라 자동 탐지가 안 되므로, 설정이 비면
	// 기동 시점에 막는다 — 런타임에 partial 로 드러나면 이미 늦다.
	if strings.TrimSpace(d.Config.KeycloakDatabase.Host) == "" {
		return nil, fmt.Errorf("backup.keycloak_database.host 가 비어 있습니다. " +
			"Keycloak DB 를 빠뜨리면 복구해도 로그인 경로가 없습니다 — " +
			"외부 IdP 를 쓰는 구성이라면 backup.enabled=false 로 두거나 값을 채우세요")
	}

	st, err := store.New(store.Config{
		Endpoint:  d.Config.Destination.Endpoint,
		AccessKey: d.Config.Destination.AccessKey,
		SecretKey: d.Config.Destination.SecretKey,
		Bucket:    d.Config.Destination.Bucket,
		Region:    d.Config.Destination.Region,
		UseSSL:    d.Config.Destination.UseSSL,
		Prefix:    d.Config.Destination.Prefix,
	})
	if err != nil {
		return nil, err
	}
	keyID := d.Config.SealKeyID
	if keyID == "" {
		keyID = "platform-key-v1"
	}
	sl, err := sealer.NewStreamSealer(keyID, key)
	if err != nil {
		return nil, err
	}

	repo := backuprepo.NewPostgresBackupRepository(d.Pool)
	dumper := backuppostgres.NewPgDumper(d.Pool)
	targets := usecase.DBTargets{
		Platform: port.DBTarget{
			Component: domain.ComponentPlatformDB,
			Host:      d.PlatformDB.Host, Port: d.PlatformDB.Port,
			Database: d.PlatformDB.Name, User: d.PlatformDB.User, Password: d.PlatformDB.Password,
		},
		Keycloak: port.DBTarget{
			Component: domain.ComponentKeycloakDB,
			Host:      d.Config.KeycloakDatabase.Host, Port: d.Config.KeycloakDatabase.Port,
			Database: d.Config.KeycloakDatabase.Name, User: d.Config.KeycloakDatabase.User,
			Password: d.Config.KeycloakDatabase.Password,
		},
	}

	policy := domain.DefaultRetentionPolicy()
	if r := d.Config.Retention; r.Daily > 0 || r.Weekly > 0 || r.Monthly > 0 {
		policy = domain.RetentionPolicy{
			Daily: r.Daily, Weekly: r.Weekly, Monthly: r.Monthly, MaxTotalBytes: r.MaxTotalBytes,
		}
	}

	m := &Module{
		Repo:      repo,
		Verify:    usecase.NewVerifyUseCase(repo, st, dumper, targets.Platform, d.Logger),
		Retention: usecase.NewRetentionUseCase(repo, st, policy, d.Logger),
		deps:      d,
		store:     st,
		sealer:    sl,
		dumper:    dumper,
		kv:        backupopenbao.NewKVExporter(d.SecretRouter),
		targets:   targets,
		tokens:    bridge.NewTokenSourceLister(d.TokenSources),
		notifier:  notify.NewLogNotifier(d.Logger),
		cache:     map[string]*backupkube.Adapters{},
	}
	d.Logger.Info("백업 모듈 준비 완료",
		"destination", d.Config.Destination.Endpoint, "bucket", d.Config.Destination.Bucket)
	return m, nil
}

// adaptersFor 는 스택이 올라간 클러스터의 어댑터를 만든다.
//
// stackID 가 비면 볼륨/리소스가 필요 없는 플랫폼 전용 백업이므로 nil 을
// 돌려준다 — 그 경로는 클러스터에 접근하지 않는다.
func (m *Module) adaptersFor(ctx context.Context, stackID string) (*backupkube.Adapters, error) {
	if strings.TrimSpace(stackID) == "" {
		return nil, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if a, ok := m.cache[stackID]; ok {
		return a, nil
	}

	clusterID, err := m.clusterIDForStack(ctx, stackID)
	if err != nil {
		return nil, err
	}
	kubeconfig, err := m.deps.Kubeconfig.GetKubeconfig(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("스택 %s 의 클러스터 kubeconfig: %w", stackID, err)
	}
	a, err := backupkube.NewAdapters(kubeconfig, backupkube.DefaultHelperImage)
	if err != nil {
		return nil, err
	}
	m.cache[stackID] = a
	return a, nil
}

func (m *Module) clusterIDForStack(ctx context.Context, stackID string) (string, error) {
	var clusterID string
	err := m.deps.Pool.QueryRow(ctx, `SELECT cluster_id::text FROM stacks WHERE id = $1`, stackID).Scan(&clusterID)
	if err != nil {
		return "", fmt.Errorf("스택 %s 의 클러스터를 찾을 수 없습니다: %w", stackID, err)
	}
	return clusterID, nil
}

// Backup 은 스택별 백업 유스케이스를 만든다 (handler.UseCaseFactory).
func (m *Module) Backup(ctx context.Context, stackID string) (*usecase.BackupUseCase, error) {
	a, err := m.adaptersFor(ctx, stackID)
	if err != nil {
		return nil, err
	}
	deps := usecase.BackupDeps{
		Repo: m.Repo, Dumper: m.dumper, KV: m.kv, Sealer: m.sealer, Store: m.store,
		Notifier: m.notifier, Targets: m.targets,
		PlatformVersion: m.deps.PlatformVersion, Logger: m.deps.Logger,
	}
	if m.deps.RotationPausing != nil {
		deps.Pauser = bridge.NewRotationPauser(m.deps.RotationPausing)
	}
	if a != nil {
		deps.Scaler, deps.Archiver, deps.Resources = a.Scaler, a.Archiver, a.Resources
		deps.Releases = a.Releases
	}
	return usecase.NewBackupUseCase(deps), nil
}

// Restore 는 스택별 복구 유스케이스를 만든다.
func (m *Module) Restore(ctx context.Context, stackID string) (*usecase.RestoreUseCase, error) {
	a, err := m.adaptersFor(ctx, stackID)
	if err != nil {
		return nil, err
	}
	deps := usecase.RestoreDeps{
		Repo: m.Repo, Dumper: m.dumper, KV: m.kv, Sealer: m.sealer, Store: m.store,
		TokenSources: m.tokens, Targets: m.targets, Logger: m.deps.Logger,
		Prereq: m.prerequisites,
	}
	if a != nil {
		deps.Scaler, deps.Archiver, deps.Resources = a.Scaler, a.Archiver, a.Resources
		deps.Releases = a.Releases
	}
	return usecase.NewRestoreUseCase(deps), nil
}

// prerequisites 는 복구 착수 전제 조건을 읽는다 (§6.1 0단계).
func (m *Module) prerequisites(ctx context.Context) domain.Prerequisites {
	p := domain.Prerequisites{
		EncryptionKeyPresent:    m.encryptionKeyPresent(),
		BackupSealKeyPresent:    len(m.deps.Config.SealKey) == 32,
		DestinationCredsPresent: m.deps.Config.Destination.AccessKey != "" && m.deps.Config.Destination.SecretKey != "",
	}
	// 도달성은 실제로 붙어 봐야 안다.
	p.DestinationReachable = m.store.Preflight(ctx, 0) == nil
	return p
}

func (m *Module) encryptionKeyPresent() bool {
	return len(m.deps.EncryptionKey) == 32
}
