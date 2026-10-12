package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sync"
	"testing"

	"github.com/cloud-nullus/draft/internal/backup/domain"
	"github.com/cloud-nullus/draft/internal/backup/port"
)

// trace 는 단계 실행 순서를 기록한다. 이 모듈의 핵심 불변식이 "무엇을 하느냐"
// 가 아니라 "어떤 순서로 하느냐" 라서, 순서를 직접 검증할 수 있어야 한다.
type trace struct {
	mu    sync.Mutex
	steps []string
}

func (t *trace) add(s string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.steps = append(t.steps, s)
}

func (t *trace) all() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string(nil), t.steps...)
}

func (t *trace) indexOf(s string) int {
	for i, v := range t.all() {
		if v == s {
			return i
		}
	}
	return -1
}

type fakeRepo struct {
	// updatedStatus 는 UpdateRun 이 실제로 받아 적은 상태의 순서다.
	updatedStatus []domain.Status
	runs          map[string]*domain.BackupRun
	restores      map[string]*domain.RestoreRun
	artifacts     map[string][]*domain.Artifact
	summaries     []domain.RunSummary
	deleted       []string
	seq           int
	mu            sync.Mutex
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		runs:      map[string]*domain.BackupRun{},
		restores:  map[string]*domain.RestoreRun{},
		artifacts: map[string][]*domain.Artifact{},
	}
}

func (f *fakeRepo) CreateRun(_ context.Context, run *domain.BackupRun) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++
	run.ID = fmt.Sprintf("run-%d", f.seq)
	f.runs[run.ID] = run
	return nil
}
func (f *fakeRepo) UpdateRun(ctx context.Context, run *domain.BackupRun) error {
	// 실제 저장소는 취소된 컨텍스트로는 쓰지 못한다.
	if ctx.Err() != nil {
		return ctx.Err()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runs[run.ID] = run
	f.updatedStatus = append(f.updatedStatus, run.Status)
	return nil
}
func (f *fakeRepo) GetRun(_ context.Context, id string) (*domain.BackupRun, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.runs[id]
	if !ok {
		return nil, domain.ErrBackupNotFound(id)
	}
	return r, nil
}
func (f *fakeRepo) ListRuns(context.Context, string, int) ([]*domain.BackupRun, error) {
	return nil, nil
}
func (f *fakeRepo) ListSummaries(context.Context, string) ([]domain.RunSummary, error) {
	return f.summaries, nil
}
func (f *fakeRepo) AddArtifact(_ context.Context, a *domain.Artifact) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.artifacts[a.BackupRunID] = append(f.artifacts[a.BackupRunID], a)
	return nil
}
func (f *fakeRepo) ListArtifacts(_ context.Context, id string) ([]*domain.Artifact, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.artifacts[id], nil
}
func (f *fakeRepo) DeleteArtifacts(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, id)
	delete(f.artifacts, id)
	return nil
}
func (f *fakeRepo) CreateRestore(_ context.Context, r *domain.RestoreRun) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++
	r.ID = fmt.Sprintf("restore-%d", f.seq)
	f.restores[r.ID] = r
	return nil
}
func (f *fakeRepo) UpdateRestore(_ context.Context, r *domain.RestoreRun) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.restores[r.ID] = r
	return nil
}
func (f *fakeRepo) GetRestore(_ context.Context, id string) (*domain.RestoreRun, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.restores[id], nil
}

type fakeStore struct {
	tr             *trace
	objects        map[string][]byte
	preflightErr   error
	putErr         error
	preflightCalls int
}

func newFakeStore(tr *trace) *fakeStore {
	return &fakeStore{tr: tr, objects: map[string][]byte{}}
}

func (f *fakeStore) Preflight(context.Context, int64) error {
	f.preflightCalls++
	f.tr.add("store.preflight")
	return f.preflightErr
}
func (f *fakeStore) Put(_ context.Context, key string, r io.Reader) (port.PutResult, error) {
	if f.putErr != nil {
		return port.PutResult{}, f.putErr
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return port.PutResult{}, err
	}
	f.objects[key] = b
	f.tr.add("store.put:" + key)
	return port.PutResult{Bytes: int64(len(b)), ChecksumSHA256: "sha-" + key, Location: "s3://backup/" + key}, nil
}
func (f *fakeStore) Get(_ context.Context, key string) (io.ReadCloser, error) {
	b, ok := f.objects[key]
	if !ok {
		return nil, errors.New("not found: " + key)
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}
func (f *fakeStore) Delete(_ context.Context, prefix string) error {
	f.tr.add("store.delete:" + prefix)
	return nil
}
func (f *fakeStore) Stat(_ context.Context, key string) (int64, string, error) {
	b, ok := f.objects[key]
	if !ok {
		return 0, "", errors.New("not found")
	}
	return int64(len(b)), "sha-" + key, nil
}

type fakeSealer struct{}

func (fakeSealer) KeyID() string { return "test-key" }
func (fakeSealer) Seal(_ context.Context, in io.Reader, out io.Writer) error {
	_, err := io.Copy(out, in)
	return err
}
func (fakeSealer) Unseal(_ context.Context, in io.Reader, out io.Writer) error {
	_, err := io.Copy(out, in)
	return err
}

type fakeDumper struct {
	tr         *trace
	schema     domain.SchemaState
	dumpErr    error
	restoreErr error
}

func (f *fakeDumper) ServerVersion(context.Context, port.DBTarget) (string, error) {
	return "17.5", nil
}
func (f *fakeDumper) Dump(_ context.Context, t port.DBTarget, out io.Writer) (port.DumpResult, error) {
	if f.dumpErr != nil {
		return port.DumpResult{}, f.dumpErr
	}
	f.tr.add("db.dump:" + string(t.Component))
	n, _ := out.Write([]byte("dump-" + t.Database))
	return port.DumpResult{ClientVersion: "17.5", BytesWritten: int64(n)}, nil
}
func (f *fakeDumper) Restore(_ context.Context, t port.DBTarget, in io.Reader) error {
	if f.restoreErr != nil {
		return f.restoreErr
	}
	_, _ = io.ReadAll(in)
	f.tr.add("db.restore:" + string(t.Component))
	return nil
}
func (f *fakeDumper) SchemaState(context.Context, port.DBTarget) (domain.SchemaState, error) {
	return f.schema, nil
}

type fakeKV struct {
	tr      *trace
	missing map[string]bool
	expErr  error
	// importFails 는 처음 N 번의 Import 를 실패시킨다(금고가 아직 안 뜬 상황).
	importFails    int
	importAttempts int
	importedStack  string
}

func (f *fakeKV) Export(_ context.Context, _ string, out io.Writer) (port.KVExportResult, error) {
	if f.expErr != nil {
		return port.KVExportResult{}, f.expErr
	}
	f.tr.add("kv.export")
	n, _ := out.Write([]byte(`{"paths":{}}`))
	return port.KVExportResult{PathCount: 3, Bytes: int64(n)}, nil
}
func (f *fakeKV) Import(_ context.Context, stackID string, in io.Reader) error {
	_, _ = io.ReadAll(in)
	f.importAttempts++
	if f.importAttempts <= f.importFails {
		f.tr.add("kv.import.failed")
		return errors.New("connection refused — 금고가 아직 뜨지 않았다")
	}
	f.importedStack = stackID
	f.tr.add("kv.import")
	return nil
}
func (f *fakeKV) PathExists(_ context.Context, _, path string) (bool, error) {
	return !f.missing[path], nil
}

type fakeScaler struct {
	tr        *trace
	workloads []domain.Workload
	scaleErr  error
	scaled    []string
}

func (f *fakeScaler) List(context.Context, string) ([]domain.Workload, error) {
	return f.workloads, nil
}
func (f *fakeScaler) Scale(ctx context.Context, t domain.QuiesceTarget, replicas int32) error {
	// 실제 어댑터는 취소된 컨텍스트로는 API 서버에 닿지 못한다.
	if ctx.Err() != nil {
		f.tr.add(fmt.Sprintf("scale.canceled:%s=%d", t.Name, replicas))
		return ctx.Err()
	}
	if f.scaleErr != nil && replicas == 0 {
		return f.scaleErr
	}
	f.scaled = append(f.scaled, fmt.Sprintf("%s=%d", t.Name, replicas))
	if replicas == 0 {
		f.tr.add("scale.down:" + t.Name)
	} else {
		f.tr.add("scale.up:" + t.Name)
	}
	return nil
}
func (f *fakeScaler) WaitStopped(context.Context, string, []domain.QuiesceTarget) error {
	f.tr.add("scale.waitStopped")
	return nil
}

type fakeArchiver struct {
	tr         *trace
	pvcs       []domain.VolumeSpec
	archiveErr error
	// onArchive 는 아카이브가 시작될 때 불린다 — 요청이 끊기는 순간을 흉내 낸다.
	onArchive func()
}

func (f *fakeArchiver) ListPVCs(context.Context, string) ([]domain.VolumeSpec, error) {
	return f.pvcs, nil
}
func (f *fakeArchiver) Archive(ctx context.Context, _, pvc string, out io.Writer) (int64, error) {
	if f.onArchive != nil {
		f.onArchive()
	}
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	if f.archiveErr != nil {
		return 0, f.archiveErr
	}
	f.tr.add("vol.archive:" + pvc)
	n, _ := out.Write([]byte("tar-" + pvc))
	return int64(n), nil
}
func (f *fakeArchiver) Restore(_ context.Context, _, pvc string, in io.Reader) error {
	_, _ = io.ReadAll(in)
	f.tr.add("vol.restore:" + pvc)
	return nil
}
func (f *fakeArchiver) EnsurePVC(_ context.Context, _ string, spec domain.VolumeSpec, _ string) error {
	f.tr.add("vol.ensure:" + spec.Name)
	return nil
}

type fakeResources struct{ tr *trace }

func (f *fakeResources) Dump(_ context.Context, ns string, out io.Writer) (int64, error) {
	f.tr.add("res.dump")
	n, _ := out.Write([]byte("resources-" + ns))
	return int64(n), nil
}
func (f *fakeResources) Apply(_ context.Context, _ string, in io.Reader) error {
	_, _ = io.ReadAll(in)
	f.tr.add("res.apply")
	return nil
}

type fakeTokenLister struct{ refs []port.TokenSourceRef }

func (f *fakeTokenLister) ListPaths(context.Context, string) ([]port.TokenSourceRef, error) {
	return f.refs, nil
}

type fakeNotifier struct {
	tr     *trace
	called []domain.Status
}

func (f *fakeNotifier) NotifyBackupResult(_ context.Context, run *domain.BackupRun) error {
	f.called = append(f.called, run.Status)
	f.tr.add("notify:" + string(run.Status))
	return nil
}

type fakePauser struct{ tr *trace }

func (f *fakePauser) Pause(context.Context) error { f.tr.add("rotation.pause"); return nil }
func (f *fakePauser) Resume(ctx context.Context) error {
	if ctx.Err() != nil {
		f.tr.add("rotation.resume:canceled")
		return ctx.Err()
	}
	f.tr.add("rotation.resume")
	return nil
}

// assertNoSecrets 는 저장된 매니페스트에 비밀값으로 보이는 키가 없는지 본다.
//
// 매니페스트만 암호화하지 않으므로(설계 §4.4), 여기에 비밀값이 새면 그대로
// 노출된다. 평시에 드러나지 않는 실수라 테스트로 고정한다.
func assertNoSecrets(t *testing.T, v map[string]any) {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("매니페스트 직렬화 실패: %v", err)
	}
	forbidden := regexp.MustCompile(`(?i)"[^"]*(password|secret|token|access_key|private_key|unseal|credential)[^"]*"\s*:`)
	if forbidden.Match(raw) {
		t.Fatalf("매니페스트에 비밀값으로 보이는 키가 있다: %s", string(raw))
	}
}

type failingDeleteStore struct{ *fakeStore }

func (f *failingDeleteStore) Delete(context.Context, string) error {
	return errors.New("스토리지 삭제 실패")
}

// dbTarget 은 테스트용 DB 대상이다.
//
// Host 를 반드시 채운다 — 비어 있으면 유스케이스가 "설정되지 않았다" 로
// 건너뛴다. 그 동작 자체는 TestRunBackup_대상이_설정되지_않으면_알려준다 가
// 따로 검증한다.
func dbTarget(comp domain.Component, database string) port.DBTarget {
	return port.DBTarget{Component: comp, Host: "db.test", Port: 5432, Database: database, User: "u", Password: "p"}
}

type fakeNamespaces struct{ tr *trace }

func (f *fakeNamespaces) EnsureNamespace(_ context.Context, namespace, _ string) error {
	f.tr.add("ns.ensure:" + namespace)
	return nil
}

type fakeGatewayAliases struct {
	tr         *trace
	ip         string
	updated    []string
	err        error
	stackID    string
	namespace  string
	kubeconfig string
}

func (f *fakeGatewayAliases) RepinGatewayAliases(_ context.Context, stackID, namespace string, kubeconfig []byte) (string, []string, error) {
	f.tr.add("gateway.repin")
	f.stackID, f.namespace, f.kubeconfig = stackID, namespace, string(kubeconfig)
	return f.ip, f.updated, f.err
}

// fakeShared 는 공용 자원 덤프·복원 대역이다. payload 가 비면 "담을 것이 없다" 다.
type fakeShared struct {
	tr       *trace
	payload  string
	applyErr error
}

func (f *fakeShared) DumpShared(_ context.Context, out io.Writer) (int64, error) {
	f.tr.add("shared.dump")
	n, _ := out.Write([]byte(f.payload))
	return int64(n), nil
}

func (f *fakeShared) ApplyShared(_ context.Context, in io.Reader) error {
	_, _ = io.ReadAll(in)
	f.tr.add("shared.apply")
	return f.applyErr
}

func (f *fakeShared) ApplySharedSecrets(_ context.Context, in io.Reader) error {
	_, _ = io.ReadAll(in)
	f.tr.add("shared.secrets")
	return nil
}

// fakeSharedComponents 는 공용 컨트롤러 보장 대역이다(스택 모듈이 구현한다).
type fakeSharedComponents struct {
	tr       *trace
	err      error
	warnings []string
}

func (f *fakeSharedComponents) EnsureSharedComponents(_ context.Context, _ []byte, _ string) ([]string, []string, error) {
	f.tr.add("components.ensure")
	if f.err != nil {
		return nil, nil, f.err
	}
	return []string{"installing_cert_manager", "installing_metrics_server"}, f.warnings, nil
}

// fakeResetter 는 DB 연결 풀 재설정 대역이다(pgxpool.Pool.Reset).
type fakeResetter struct{ tr *trace }

func (f *fakeResetter) Reset() { f.tr.add("db.reset") }
