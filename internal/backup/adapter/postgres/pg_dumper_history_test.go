package postgres

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/backup/domain"
	"github.com/cloud-nullus/draft/internal/backup/port"
)

// postgres:18 에 000075·000076 과 이력과 무관한 테이블(stacks)을 올려 뜬 덤프의 목차다
// (pg_restore -l, 실측).
const realTOC = `;
; Archive created at 2026-10-11 03:30:00 UTC
;     dbname: nullus
;
220; 1259 16407 TABLE public backup_artifacts nullus
219; 1259 16385 TABLE public backup_runs nullus
221; 1259 16434 TABLE public restore_runs nullus
223; 1259 16467 TABLE public schema_migrations nullus
222; 1259 16458 TABLE public stacks nullus
3484; 0 16407 TABLE DATA public backup_artifacts nullus
3483; 0 16385 TABLE DATA public backup_runs nullus
3485; 0 16434 TABLE DATA public restore_runs nullus
3487; 0 16467 TABLE DATA public schema_migrations nullus
3486; 0 16458 TABLE DATA public stacks nullus
3324; 2606 16427 CONSTRAINT public backup_artifacts backup_artifacts_pkey nullus
3320; 2606 16404 CONSTRAINT public backup_runs backup_runs_pkey nullus
3328; 2606 16451 CONSTRAINT public restore_runs restore_runs_pkey nullus
3333; 2606 16472 CONSTRAINT public schema_migrations schema_migrations_pkey nullus
3331; 2606 16465 CONSTRAINT public stacks stacks_pkey nullus
3325; 1259 16433 INDEX public idx_backup_artifacts_run nullus
3321; 1259 16405 INDEX public idx_backup_runs_org_created nullus
3322; 1259 16406 INDEX public idx_backup_runs_status_created nullus
3326; 1259 16457 INDEX public idx_restore_runs_backup nullus
3329; 1259 16466 INDEX public idx_stacks_name nullus
3334; 2606 16428 FK CONSTRAINT public backup_artifacts backup_artifacts_backup_run_id_fkey nullus
3335; 2606 16452 FK CONSTRAINT public restore_runs restore_runs_backup_run_id_fkey nullus
`

var historyIndexes = []string{
	"idx_backup_artifacts_run", "idx_backup_runs_org_created",
	"idx_backup_runs_status_created", "idx_restore_runs_backup",
}

// 복구가 목록에서 빼는 것은 이력 테이블에 딸린 항목뿐이다. 하나라도 남으면 pg_restore --clean 이
// 지금의 이력을 지우고 백업 시점의 것으로 바꾼다 — 그 뒤의 백업이 목록에서 사라지고, 진행 중인
// 복구의 기록까지 지워져 404 가 된다.
func TestFilterTOC_DropsOnlyHistoryEntries(t *testing.T) {
	got := filterTOC(realTOC, domain.HistoryTables, historyIndexes)

	for _, name := range []string{"backup_runs", "backup_artifacts", "restore_runs"} {
		assert.NotContains(t, got, " "+name+" ", "이력 테이블 항목이 남았다: %s", name)
	}
	for _, idx := range historyIndexes {
		assert.NotContains(t, got, idx)
	}
	for _, keep := range []string{
		"TABLE public stacks nullus", "TABLE DATA public stacks nullus",
		"CONSTRAINT public stacks stacks_pkey", "INDEX public idx_stacks_name",
		"TABLE public schema_migrations", "TABLE DATA public schema_migrations",
	} {
		assert.Contains(t, got, keep, "이력과 무관한 항목은 그대로 되돌린다")
	}
	// pg_restore -L 은 목록 순서를 따른다. 거른 뒤에도 원래 순서를 지킨다.
	assert.Less(t, strings.Index(got, "TABLE public stacks"), strings.Index(got, "TABLE DATA public stacks"))
}

// 보존할 테이블이 없으면(Keycloak DB) 목록을 건드리지 않는다.
func TestFilterTOC_NoPreservedTables(t *testing.T) {
	assert.Equal(t, realTOC, filterTOC(realTOC, nil, nil))
}

// 덤프는 이력 테이블도 담는다. 플랫폼 DB 를 통째로 잃었을 때 이전 백업들의 카탈로그(매니페스트·
// 산출물 위치)를 되살릴 사본이 이 덤프뿐이다 — 버킷에는 매니페스트가 없다. 지키는 일은 복구가 한다.
func TestDump_KeepsHistoryForDisasterRecovery(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args")
	fake := writeScript(t, dir, "pg_dump", `
if [ "$1" = "--version" ]; then echo "pg_dump (PostgreSQL) 18.3"; exit 0; fi
echo "$@" > '`+argsFile+`'
printf 'PGDMP'
`)
	d := &PgDumper{dumpBin: fake}

	var out strings.Builder
	_, err := d.Dump(context.Background(), port.DBTarget{
		Component: domain.ComponentPlatformDB, Database: "nullus", PreserveTables: domain.HistoryTables,
	}, &out)
	require.NoError(t, err)

	args, _ := os.ReadFile(argsFile)
	assert.NotContains(t, string(args), "--exclude-table")
	assert.Equal(t, "PGDMP", out.String())
}

// 이 변경 전에 뜬 덤프에는 이력 테이블이 들어 있다. 복구는 목차에서 그 항목을 빼고 되돌린다.
func TestRestore_SkipsPreservedTablesFromArchive(t *testing.T) {
	dir := t.TempDir()
	listCopy := filepath.Join(dir, "list-used")
	argsFile := filepath.Join(dir, "args")
	// -l 이면 목차를 내고, 그 밖에는 받은 목록 파일을 남긴다.
	fake := writeScript(t, dir, "pg_restore", `
for a in "$@"; do
  if [ "$a" = "-l" ]; then cat <<'TOC'
`+realTOC+`TOC
exit 0; fi
done
echo "$@" >> '`+argsFile+`'
prev=""
for a in "$@"; do
  if [ "$prev" = "-L" ]; then cp "$a" '`+listCopy+`'; fi
  prev="$a"
done
`)
	d := &PgDumper{restoreBin: fake}
	d.indexNames = func(context.Context, port.DBTarget, []string) ([]string, error) { return historyIndexes, nil }

	err := d.Restore(context.Background(), port.DBTarget{
		Component: domain.ComponentPlatformDB, Database: "nullus", PreserveTables: domain.HistoryTables,
	}, strings.NewReader("PGDMP-archive"))
	require.NoError(t, err)

	used, err := os.ReadFile(listCopy)
	require.NoError(t, err, "복구가 거른 목록(-L)을 넘기지 않았다")
	assert.NotContains(t, string(used), "backup_runs")
	assert.Contains(t, string(used), "TABLE DATA public stacks")

	args, _ := os.ReadFile(argsFile)
	for _, want := range []string{"--clean", "--if-exists", "--single-transaction", "-L"} {
		assert.Contains(t, string(args), want)
	}
}

func writeScript(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755))
	return path
}

// 이력 테이블이 플랫폼 테이블을 FK 로 참조하면 --clean 이 그 PK 를 지우지 못해 복원이 죽는다
// (2026-10-12 리허설, 000091 의 backup_runs_org_id_fkey). pg_restore 의 원문만으로는 무엇을 고쳐야
// 하는지 보이지 않는다 — 원인과 조치를 덧붙인다.
func TestRestore_이력_테이블_FK_가_막으면_무엇을_고칠지_말한다(t *testing.T) {
	dir := t.TempDir()
	fake := writeScript(t, dir, "pg_restore", `
for a in "$@"; do
  if [ "$a" = "-l" ]; then cat <<'TOC'
`+realTOC+`TOC
exit 0; fi
done
echo 'pg_restore: error: could not execute query: ERROR:  cannot drop constraint organizations_pkey on table public.organizations because other objects depend on it' >&2
echo 'DETAIL:  constraint backup_runs_org_id_fkey on table public.backup_runs depends on index public.organizations_pkey' >&2
exit 1
`)
	d := &PgDumper{restoreBin: fake}
	d.indexNames = func(context.Context, port.DBTarget, []string) ([]string, error) { return historyIndexes, nil }

	err := d.Restore(context.Background(), port.DBTarget{
		Component: domain.ComponentPlatformDB, Database: "nullus", PreserveTables: domain.HistoryTables,
	}, strings.NewReader("PGDMP-archive"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "backup_runs_org_id_fkey", "원문은 남긴다")
	assert.Contains(t, err.Error(), "backup_runs", "어느 이력 테이블인지")
	assert.Contains(t, err.Error(), "FK", "원인 — 이력 테이블의 바깥 FK")
	assert.Contains(t, err.Error(), "마이그레이션", "조치 — FK 를 거두는 마이그레이션")
}

func TestRestore_다른_실패는_원문_그대로다(t *testing.T) {
	dir := t.TempDir()
	fake := writeScript(t, dir, "pg_restore", `echo 'pg_restore: error: connection to server failed' >&2; exit 1`)
	d := &PgDumper{restoreBin: fake}

	err := d.Restore(context.Background(), port.DBTarget{Component: domain.ComponentKeycloakDB, Database: "keycloak"},
		strings.NewReader("PGDMP-archive"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection to server failed")
	assert.NotContains(t, err.Error(), "FK")
}
