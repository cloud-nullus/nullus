//go:build rehearsal

// 복구 리허설 (B4-1). 설계 §10.2 (nullus-plan#75)
//
// 이 EPIC 의 완료 기준이다. 팀에 "백업은 되지만 복원이 잘 안 됐던 경험"
// (2026-07-19 회의)이 있고, 그것이 이 EPIC 의 존재 이유다.
//
// 검증 방식이 핵심이다 — "GitLab 이 뜬다" 는 복원 검증이 아니다. 백업 전에
// 심어 둔 **지문**(파일 내용 해시 · replica 수 · DB 행)이 복원 후에도 같은
// 값으로 있는가, 그것만이 "그 상태 그대로" 의 증거다.
//
// 축소 리허설이다. 실제 OSS(GitLab/Harbor/Jenkins) 대신 PVC 를 잡는 워크로드
// 하나를 쓴다. 검증하는 것은 **메커니즘**이다 — 정지 → 볼륨 아카이브 →
// 네임스페이스 파괴 → PVC 재생성 → 볼륨 복원 → 워크로드 재개. 도구별 특성
// (Gitaly 정합성 등)은 실환경 리허설의 몫으로 남는다.
//
// 실행:
//
//	kind get kubeconfig --name <cluster> > /tmp/kc.yaml
//	export NULLUS_REHEARSAL_KUBECONFIG=/tmp/kc.yaml
//	export PATH="/opt/homebrew/opt/libpq/bin:$PATH"   # pg_dump
//	go test -tags rehearsal ./internal/backup/rehearsal/ -v -timeout 20m
package rehearsal

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	backupkube "github.com/cloud-nullus/draft/internal/backup/adapter/kube"
	backuppostgres "github.com/cloud-nullus/draft/internal/backup/adapter/postgres"
	backuprepo "github.com/cloud-nullus/draft/internal/backup/adapter/repository"
	"github.com/cloud-nullus/draft/internal/backup/adapter/sealer"
	"github.com/cloud-nullus/draft/internal/backup/adapter/store"
	"github.com/cloud-nullus/draft/internal/backup/domain"
	"github.com/cloud-nullus/draft/internal/backup/port"
	"github.com/cloud-nullus/draft/internal/backup/usecase"
)

const (
	rehearsalNS = "nullus-backup-rehearsal"
	pvcName     = "rehearsal-data"
	deployName  = "rehearsal-app"
	appReplicas = int32(2)

	minioImage = "quay.io/minio/minio:RELEASE.2024-12-18T13-15-44Z"
	helperImg  = "busybox:1.37"
)

// fingerprint 는 복원 후 대조할 증거다.
type fingerprint struct {
	FileName string
	SHA256   string
	Bytes    int
	Replicas int32
	DBRows   int
}

func TestRecoveryRehearsal(t *testing.T) {
	kubeconfigPath := os.Getenv("NULLUS_REHEARSAL_KUBECONFIG")
	if kubeconfigPath == "" {
		t.Skip("NULLUS_REHEARSAL_KUBECONFIG 가 없어 건너뜁니다 (실제 클러스터가 필요합니다)")
	}
	kubeconfig, err := os.ReadFile(kubeconfigPath)
	require.NoError(t, err)

	ctx := context.Background()
	restCfg, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	require.NoError(t, err)
	client, err := kubernetes.NewForConfig(restCfg)
	require.NoError(t, err)

	// 이 리허설은 전용 네임스페이스만 쓰고 끝나면 지운다.
	t.Cleanup(func() { deleteNamespace(context.Background(), client, rehearsalNS) })

	pool, pgTarget, kcTarget, pgCleanup := setupPlatformDB(t)
	t.Cleanup(pgCleanup)
	objStore, mcCleanup := setupDestination(t)
	t.Cleanup(mcCleanup)

	var timings []string
	step := func(name string, fn func()) {
		start := time.Now()
		fn()
		d := time.Since(start).Round(time.Millisecond)
		timings = append(timings, fmt.Sprintf("%-28s %s", name, d))
		t.Logf("[%s] %s", d, name)
	}

	// ── 1. 기준 환경 구성 + 지문 심기 ───────────────────────────────────
	var fp fingerprint
	step("1. 기준 환경 구성", func() {
		fp = seedEnvironment(t, ctx, client, restCfg, pool)
	})
	t.Logf("지문: file=%s sha256=%s replicas=%d db_rows=%d",
		fp.FileName, fp.SHA256[:16]+"…", fp.Replicas, fp.DBRows)

	repo := backuprepo.NewPostgresBackupRepository(pool)
	dumper := backuppostgres.NewPgDumper(pool)
	sl, err := sealer.NewStreamSealer("rehearsal-key", mustKey())
	require.NoError(t, err)
	adapters, err := backupkube.NewAdapters(kubeconfig, helperImg)
	require.NoError(t, err)
	// 통합 배치(§1.2 결정 1)를 그대로 재현한다 — 한 인스턴스, 두 database.
	// 복원이 database 단위인지(§6.6)도 이 구성에서만 확인된다.
	targets := usecase.DBTargets{Platform: pgTarget, Keycloak: kcTarget}

	backupUC := usecase.NewBackupUseCase(usecase.BackupDeps{
		Repo: repo, Dumper: dumper, KV: noopKV{}, Scaler: adapters.Scaler,
		Archiver: adapters.Archiver, Resources: adapters.Resources,
		Sealer: sl, Store: objStore, Targets: targets, PlatformVersion: "rehearsal",
	})

	// ── 2. 백업 ─────────────────────────────────────────────────────────
	var run *domain.BackupRun
	step("2. 백업 실행", func() {
		run, err = backupUC.Run(ctx, usecase.RunBackupRequest{
			OrgID:     "00000000-0000-0000-0000-000000000001",
			Namespace: rehearsalNS, Mode: domain.ModeFull,
		})
		require.NoError(t, err)
	})
	require.Equal(t, domain.StatusSucceeded, run.Status, "백업이 성공해야 다음이 의미 있다: %s", run.Error)
	require.NotNil(t, run.QuiesceStartedAt, "정지 창이 열렸어야 한다")
	quiesce := run.QuiesceEndedAt.Sub(*run.QuiesceStartedAt).Round(time.Millisecond)
	t.Logf("실제 정지 창: %s", quiesce)

	arts, err := repo.ListArtifacts(ctx, run.ID)
	require.NoError(t, err)
	t.Logf("산출물 %d 건, 총 %d bytes", len(arts), run.TotalBytes)
	assertHasComponent(t, arts, domain.ComponentVolume)
	assertHasComponent(t, arts, domain.ComponentNamespaceResources)
	assertHasComponent(t, arts, domain.ComponentPlatformDB)

	// ── 3. 환경 초기화 — 부분 초기화 금지 ───────────────────────────────
	step("3. 환경 초기화", func() {
		deleteNamespace(ctx, client, rehearsalNS)
		waitNamespaceGone(t, ctx, client, rehearsalNS)

		// 플랫폼 DB 도 실제로 잃은 상태를 만든다.
		_, err := pool.Exec(ctx, `DROP TABLE IF EXISTS rehearsal_fingerprint`)
		require.NoError(t, err)
	})

	// 초기화가 진짜인지 확인한다 — 이걸 빼면 "복구했다" 가 착각일 수 있다.
	_, err = client.CoreV1().PersistentVolumeClaims(rehearsalNS).Get(ctx, pvcName, metav1.GetOptions{})
	require.Error(t, err, "PVC 가 실제로 사라졌어야 한다")

	// ── 4. 복구 ─────────────────────────────────────────────────────────
	restoreUC := usecase.NewRestoreUseCase(usecase.RestoreDeps{
		Repo: repo, Dumper: dumper, KV: noopKV{}, Scaler: adapters.Scaler,
		Archiver: adapters.Archiver, Resources: adapters.Resources,
		Sealer: sl, Store: objStore, Targets: targets,
		Prereq: func(context.Context) domain.Prerequisites {
			return domain.Prerequisites{
				EncryptionKeyPresent: true, BackupSealKeyPresent: true,
				DestinationCredsPresent: true, DestinationReachable: true,
			}
		},
	})

	require.NoError(t, ensureNamespace(ctx, client, rehearsalNS))

	var rr *domain.RestoreRun
	step("4. 복구 실행", func() {
		rr, err = restoreUC.Run(ctx, usecase.RunRestoreRequest{
			BackupRunID: run.ID, Namespace: rehearsalNS, Mode: domain.ModeFull,
		})
		require.NoError(t, err)
	})
	require.Equal(t, domain.StatusSucceeded, rr.Status, "복구 실패: %s", rr.Error)
	assert.True(t, rr.SchemaCheck.Allowed)

	// ── 5. 지문 대조 ────────────────────────────────────────────────────
	step("5. 워크로드 기동 대기", func() {
		// 복구는 워크로드를 되살려 놓고 끝난다. 실제로 뜨는 것까지가
		// "복구됐다" 이므로 여기서 기다린다.
		waitDeploymentReady(t, ctx, client, rehearsalNS, deployName)
	})

	step("6. 지문 대조", func() {
		// ① 볼륨 내용이 바이트 단위로 같은가
		gotSize := fileSizeInPod(t, ctx, client, rehearsalNS, deployName, "/data/"+fp.FileName)
		assert.Equal(t, fp.Bytes, gotSize, "복원된 파일이 잘렸다")

		gotSum := fileSHA256InPod(t, ctx, client, rehearsalNS, deployName, "/data/"+fp.FileName)
		assert.Equal(t, fp.SHA256, gotSum,
			"복원된 파일 내용이 백업 시점과 달라졌다 — '그 상태 그대로' 가 아니다")

		// ② 워크로드가 원래 replica 로 돌아왔는가
		d, err := client.AppsV1().Deployments(rehearsalNS).Get(ctx, deployName, metav1.GetOptions{})
		require.NoError(t, err, "Deployment 가 복원됐어야 한다")
		require.NotNil(t, d.Spec.Replicas)
		assert.Equal(t, fp.Replicas, *d.Spec.Replicas,
			"정지 전 replica 로 되돌아와야 한다")

		// ③ 플랫폼 DB 행이 살아났는가
		var rows int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM rehearsal_fingerprint`).Scan(&rows))
		assert.Equal(t, fp.DBRows, rows)
	})

	t.Log("── 리허설 소요 시간 ──")
	for _, l := range timings {
		t.Log("  " + l)
	}
	t.Logf("  %-28s %s", "그중 실제 정지 창", quiesce)
}

// ── 환경 구성 ───────────────────────────────────────────────────────────

func seedEnvironment(t *testing.T, ctx context.Context, c kubernetes.Interface, cfg *rest.Config, pool *pgxpool.Pool) fingerprint {
	t.Helper()
	require.NoError(t, ensureNamespace(ctx, c, rehearsalNS))

	// PVC
	_, err := c.CoreV1().PersistentVolumeClaims(rehearsalNS).Create(ctx, &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: pvcName, Namespace: rehearsalNS},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("64Mi")},
			},
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	// 볼륨을 잡는 워크로드. 정지 대상이 된다.
	replicas := appReplicas
	_, err = c.AppsV1().Deployments(rehearsalNS).Create(ctx, &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: deployName, Namespace: rehearsalNS},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": deployName}},
			Strategy: appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": deployName}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name: "app", Image: helperImg,
						Command:      []string{"sh", "-c", "sleep 86400"},
						VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/data"}},
					}},
					Volumes: []corev1.Volume{{
						Name: "data",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName},
						},
					}},
				},
			},
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)
	// replica 2 는 RWO 볼륨이라 1개만 뜬다 — 정지/재개 검증에는 충분하다.
	waitDeploymentReady(t, ctx, c, rehearsalNS, deployName)

	// 지문 파일은 **파드 안에서** 만들고 **파드 안에서** 해시한다.
	//
	// kubectl exec 의 stdin 으로 밀어넣지 않는 이유: 그 경로가 조용히 자르면
	// 리허설이 자기 기준선조차 못 세우는데, 그것을 백업의 문제로 오해하게
	// 된다. 실제로 첫 시도에서 그렇게 헷갈렸다.
	const payloadBytes = 256 * 1024
	execInPod(t, ctx, c, rehearsalNS, deployName,
		fmt.Sprintf("head -c %d /dev/urandom > /data/fingerprint.bin && sync", payloadBytes))

	seedSum := fileSHA256InPod(t, ctx, c, rehearsalNS, deployName, "/data/fingerprint.bin")
	seedSize := fileSizeInPod(t, ctx, c, rehearsalNS, deployName, "/data/fingerprint.bin")
	require.Equal(t, payloadBytes, seedSize, "기준선을 세우지 못하면 리허설이 성립하지 않는다")

	// 플랫폼 DB 지문.
	_, err = pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS rehearsal_fingerprint (id serial primary key, note text)`)
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		_, err = pool.Exec(ctx, `INSERT INTO rehearsal_fingerprint (note) VALUES ($1)`, fmt.Sprintf("row-%d", i))
		require.NoError(t, err)
	}

	return fingerprint{
		FileName: "fingerprint.bin",
		SHA256:   seedSum,
		Bytes:    seedSize,
		Replicas: appReplicas,
		DBRows:   3,
	}
}

// ── 보조 ────────────────────────────────────────────────────────────────

func mustKey() []byte {
	k := make([]byte, 32)
	copy(k, []byte("rehearsal-32byte-key-0123456789!"))
	return k
}

// noopKV — 이 리허설은 볼륨/리소스/DB 메커니즘을 검증한다. 금고는 실제
// OpenBao 가 필요하므로 실환경 리허설의 몫이다.
type noopKV struct{}

func (noopKV) Export(context.Context, string, io.Writer) (port.KVExportResult, error) {
	return port.KVExportResult{}, nil
}
func (noopKV) Import(context.Context, string, io.Reader) error          { return nil }
func (noopKV) PathExists(context.Context, string, string) (bool, error) { return true, nil }

func assertHasComponent(t *testing.T, arts []*domain.Artifact, c domain.Component) {
	t.Helper()
	for _, a := range arts {
		if a.Component == c {
			return
		}
	}
	t.Fatalf("산출물에 %s 가 없다", c)
}

// ensureNamespace 는 종료 중인 네임스페이스가 완전히 사라질 때까지 기다린 뒤
// 만든다. 앞선 리허설의 잔재 위에 새 환경을 세우면 검증이 무의미해진다.
func ensureNamespace(ctx context.Context, c kubernetes.Interface, ns string) error {
	deadline := time.Now().Add(3 * time.Minute)
	for {
		cur, err := c.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
		if err != nil {
			break // 없다 — 만들면 된다
		}
		if cur.Status.Phase != corev1.NamespaceTerminating {
			return nil // 이미 쓸 수 있다
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("네임스페이스 %s 가 종료되지 않습니다", ns)
		}
		time.Sleep(2 * time.Second)
	}

	_, err := c.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: ns},
	}, metav1.CreateOptions{})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return err
	}
	return nil
}

func deleteNamespace(ctx context.Context, c kubernetes.Interface, ns string) {
	_ = c.CoreV1().Namespaces().Delete(ctx, ns, metav1.DeleteOptions{})
}

func waitNamespaceGone(t *testing.T, ctx context.Context, c kubernetes.Interface, ns string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		if _, err := c.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{}); err != nil {
			return
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatal("네임스페이스가 사라지지 않았다 — 초기화가 완전하지 않으면 복구 검증이 의미 없다")
}

func waitDeploymentReady(t *testing.T, ctx context.Context, c kubernetes.Interface, ns, name string) {
	t.Helper()
	deadline := time.Now().Add(4 * time.Minute)
	for time.Now().Before(deadline) {
		d, err := c.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
		if err == nil && d.Status.ReadyReplicas >= 1 {
			return
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatal("워크로드가 기동하지 않았다")
}

func podOf(t *testing.T, ctx context.Context, c kubernetes.Interface, ns, app string) string {
	t.Helper()
	pods, err := c.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: "app=" + app})
	require.NoError(t, err)
	for _, p := range pods.Items {
		if p.Status.Phase == corev1.PodRunning {
			return p.Name
		}
	}
	t.Fatal("실행 중인 파드를 찾지 못했다")
	return ""
}

// 아래 헬퍼들은 **파드 안에서** 명령을 돌리고 짧은 결과만 받아온다.
//
// 큰 데이터를 kubectl 로 밀거나 빼지 않는 것이 요점이다 — 그 경로가 조용히
// 자르면 검증이 백업을 잘못 고발한다. 어댑터와 경로를 공유하지도 않는다:
// 어댑터를 재사용하면 어댑터의 버그가 검증까지 함께 속인다.
func execInPod(t *testing.T, ctx context.Context, c kubernetes.Interface, ns, app, script string) string {
	t.Helper()
	pod := podOf(t, ctx, c, ns, app)
	cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", os.Getenv("NULLUS_REHEARSAL_KUBECONFIG"),
		"-n", ns, "exec", pod, "--", "sh", "-c", script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	require.NoError(t, cmd.Run(), "파드 명령 실패(%s): %s", script, stderr.String())
	return strings.TrimSpace(stdout.String())
}

func fileSHA256InPod(t *testing.T, ctx context.Context, c kubernetes.Interface, ns, app, path string) string {
	t.Helper()
	out := execInPod(t, ctx, c, ns, app, "sha256sum "+path)
	return strings.Fields(out)[0]
}

func fileSizeInPod(t *testing.T, ctx context.Context, c kubernetes.Interface, ns, app, path string) int {
	t.Helper()
	out := execInPod(t, ctx, c, ns, app, "wc -c < "+path)
	n, err := strconv.Atoi(strings.TrimSpace(out))
	require.NoError(t, err, "크기를 읽지 못했다: %q", out)
	return n
}

func setupPlatformDB(t *testing.T) (*pgxpool.Pool, port.DBTarget, port.DBTarget, func()) {
	t.Helper()
	ctx := context.Background()
	c, err := tcpostgres.Run(ctx, "postgres:18",
		tcpostgres.WithDatabase("nullus"),
		tcpostgres.WithUsername("nullus"),
		tcpostgres.WithPassword("nullus_dev"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2)),
	)
	require.NoError(t, err)

	connStr, err := c.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)

	applyBackupSchema(t, ctx, pool)

	// 같은 인스턴스에 Keycloak 용 database 를 하나 더 만든다.
	_, err = pool.Exec(ctx, `CREATE DATABASE keycloak`)
	require.NoError(t, err)

	host, _ := c.Host(ctx)
	port5432, _ := c.MappedPort(ctx, "5432")
	mk := func(comp domain.Component, db string) port.DBTarget {
		return port.DBTarget{
			Component: comp, Host: host, Port: port5432.Int(),
			Database: db, User: "nullus", Password: "nullus_dev",
		}
	}
	return pool, mk(domain.ComponentPlatformDB, "nullus"), mk(domain.ComponentKeycloakDB, "keycloak"),
		func() {
			pool.Close()
			_ = c.Terminate(context.Background())
		}
}

func applyBackupSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	sqlBytes, err := os.ReadFile("../../../db/migrations/000075_backup_restore.up.sql")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(sqlBytes))
	require.NoError(t, err)
	// 스키마 버전 검사가 참조한다.
	_, err = pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version bigint primary key, dirty boolean not null)`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO schema_migrations (version, dirty) VALUES (75, false) ON CONFLICT DO NOTHING`)
	require.NoError(t, err)
}

func setupDestination(t *testing.T) (*store.S3Store, func()) {
	t.Helper()
	ctx := context.Background()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        minioImage,
			ExposedPorts: []string{"9000/tcp"},
			Cmd:          []string{"server", "/data"},
			Env: map[string]string{
				"MINIO_ROOT_USER": "nullus-admin", "MINIO_ROOT_PASSWORD": "nullus-minio-secret",
			},
			WaitingFor: wait.ForHTTP("/minio/health/live").WithPort("9000/tcp").
				WithStartupTimeout(2 * time.Minute),
		},
		Started: true,
	})
	require.NoError(t, err)

	host, _ := c.Host(ctx)
	p, _ := c.MappedPort(ctx, "9000")
	endpoint := fmt.Sprintf("%s:%s", host, p.Port())

	cl, err := minio.New(endpoint, &minio.Options{
		Creds: credentials.NewStaticV4("nullus-admin", "nullus-minio-secret", ""),
	})
	require.NoError(t, err)
	require.NoError(t, cl.MakeBucket(ctx, "nullus-backup", minio.MakeBucketOptions{}))

	s, err := store.New(store.Config{
		Endpoint: endpoint, AccessKey: "nullus-admin", SecretKey: "nullus-minio-secret",
		Bucket: "nullus-backup", Region: "us-east-1",
	})
	require.NoError(t, err)
	return s, func() { _ = c.Terminate(context.Background()) }
}
