package postgres

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/backup/domain"
	"github.com/cloud-nullus/draft/internal/backup/port"
)

// PgDumper 는 pg_dump/pg_restore 를 exec 한다.
//
// 바이너리는 API 이미지에 함께 담긴다(Dockerfile 의 postgresql17-client) —
// helm·kubectl·migrate 와 같은 방식이다.
type PgDumper struct {
	pool       *pgxpool.Pool
	dumpBin    string
	restoreBin string
	psqlBin    string
	// indexNames 는 보존할 테이블의 인덱스 이름을 읽는다. 복구 목차의 INDEX 항목에는 테이블
	// 이름이 없어 이것으로 가른다. 테스트가 바꿔 끼운다.
	indexNames func(ctx context.Context, target port.DBTarget, tables []string) ([]string, error)
}

func NewPgDumper(pool *pgxpool.Pool) *PgDumper {
	d := &PgDumper{pool: pool, dumpBin: "pg_dump", restoreBin: "pg_restore", psqlBin: "psql"}
	d.indexNames = d.queryIndexNames
	return d
}

var versionRe = regexp.MustCompile(`(\d+(?:\.\d+)*)`)

// dsn 은 대상 DB 의 접속 문자열을 만든다.
func dsn(t port.DBTarget) string {
	port := t.Port
	if port == 0 {
		port = 5432
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		url.QueryEscape(t.User), url.QueryEscape(t.Password), t.Host, port, t.Database)
}

// queryTarget 은 **대상 DB 에 직접 붙어** 한 줄을 읽는다.
//
// 공용 풀(플랫폼 DB)로 대신 읽지 않는 이유: 대상이 Keycloak DB 처럼 다른
// 호스트·다른 버전일 수 있다. 풀로 읽으면 플랫폼 DB 의 답을 대상의 답인 것처럼
// 쓰게 되고, 버전 호환성 검사와 매니페스트 기록이 조용히 틀어진다.
//
// Host 가 비면(로컬/테스트) 풀로 떨어진다.
func (d *PgDumper) queryTarget(ctx context.Context, target port.DBTarget, sql string, dest ...any) error {
	if strings.TrimSpace(target.Host) == "" {
		if d.pool == nil {
			return fmt.Errorf("대상 호스트도 DB 풀도 없습니다")
		}
		return d.pool.QueryRow(ctx, sql).Scan(dest...)
	}

	conn, err := pgx.Connect(ctx, dsn(target))
	if err != nil {
		return fmt.Errorf("%s 에 연결: %w", target.Database, err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()
	return conn.QueryRow(ctx, sql).Scan(dest...)
}

// ServerVersion 은 **대상 DB** 의 서버 버전을 돌려준다.
func (d *PgDumper) ServerVersion(ctx context.Context, target port.DBTarget) (string, error) {
	var v string
	if err := d.queryTarget(ctx, target, "SHOW server_version", &v); err != nil {
		return "", fmt.Errorf("서버 버전 조회(%s): %w", target.Database, err)
	}
	if m := versionRe.FindString(v); m != "" {
		return m, nil
	}
	return v, nil
}

// clientVersion 은 로컬 pg_dump 의 버전을 돌려준다.
func (d *PgDumper) clientVersion(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, d.dumpBin, "--version").Output()
	if err != nil {
		return "", fmt.Errorf("pg_dump 를 실행할 수 없습니다 (이미지에 postgresql-client 가 있는지 확인): %w", err)
	}
	if m := versionRe.FindString(string(out)); m != "" {
		return m, nil
	}
	return "", fmt.Errorf("pg_dump 버전을 해석할 수 없습니다: %q", strings.TrimSpace(string(out)))
}

// ensureCompatible 은 클라이언트가 서버보다 낮지 않은지 본다.
//
// 낮으면 pg_dump 가 거부하는데, 그 실패가 늦게 드러나면 "백업이 있다고
// 믿는" 상태가 된다 (§9 F6). 그래서 먼저 막는다.
func (d *PgDumper) ensureCompatible(ctx context.Context, target port.DBTarget) (string, error) {
	client, err := d.clientVersion(ctx)
	if err != nil {
		return "", err
	}
	server, err := d.ServerVersion(ctx, target)
	if err != nil {
		// 서버 버전을 못 읽으면 검사를 건너뛴다 — 백업 자체를 막지는 않는다.
		return client, nil
	}
	ok, err := IsCompatible(client, server)
	if err != nil {
		// 버전을 해석하지 못하면 검사를 건너뛴다 — 백업 자체를 막지는 않는다.
		return client, nil
	}
	if !ok {
		return "", fmt.Errorf(
			"pg_dump major 버전(%s)이 서버(%s)보다 낮습니다. 이 조합은 백업을 조용히 실패시킵니다", client, server)
	}
	return client, nil
}

func (d *PgDumper) env(target port.DBTarget) []string {
	return []string{"PGPASSWORD=" + target.Password}
}

func (d *PgDumper) connArgs(target port.DBTarget) []string {
	args := []string{"--host=" + target.Host, "--dbname=" + target.Database}
	if target.Port > 0 {
		args = append(args, fmt.Sprintf("--port=%d", target.Port))
	}
	if target.User != "" {
		args = append(args, "--username="+target.User)
	}
	return args
}

// Dump 는 custom format(-Fc) 으로 뜬다. 압축이 내장돼 있고 pg_restore 로
// 선택적 복원이 가능하다 (§3.1).
func (d *PgDumper) Dump(ctx context.Context, target port.DBTarget, out io.Writer) (port.DumpResult, error) {
	client, err := d.ensureCompatible(ctx, target)
	if err != nil {
		return port.DumpResult{}, err
	}

	// 보존할 테이블(백업 이력)도 덤프에 담는다. 플랫폼 DB 를 통째로 잃으면 이전 백업들의
	// 카탈로그(매니페스트·산출물 위치)를 되살릴 사본이 이 덤프뿐이다 — 버킷에는 매니페스트가 없다.
	// 그 테이블을 지키는 일은 Restore 가 한다.
	args := append([]string{"--format=custom", "--no-password"}, d.connArgs(target)...)
	cmd := exec.CommandContext(ctx, d.dumpBin, args...)
	cmd.Env = append(cmd.Environ(), d.env(target)...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return port.DumpResult{}, err
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return port.DumpResult{}, err
	}
	n, copyErr := io.Copy(out, bufio.NewReaderSize(stdout, 1<<20))
	waitErr := cmd.Wait()
	if copyErr != nil {
		return port.DumpResult{}, copyErr
	}
	if waitErr != nil {
		return port.DumpResult{}, fmt.Errorf("pg_dump 실패: %w (%s)", waitErr, strings.TrimSpace(stderr.String()))
	}
	return port.DumpResult{ClientVersion: client, BytesWritten: n}, nil
}

// Restore 는 database 단위로 복원한다.
//
// 인스턴스를 통째로 되돌리지 않는다 — 통합(§1.2) 이후에는 한 인스턴스에 두
// database 가 있어서, 한쪽을 복원하다 다른 쪽을 날릴 수 있다 (§6.6).
func (d *PgDumper) Restore(ctx context.Context, target port.DBTarget, in io.Reader) error {
	args := append([]string{
		"--no-password",
		// 기존 객체를 정리하고 넣는다. 남은 객체가 있으면 복원이 중간에 깨진다.
		"--clean", "--if-exists",
		// 한 트랜잭션으로 돌려 실패 시 부분 적용을 남기지 않는다.
		"--single-transaction",
	}, d.connArgs(target)...)

	if len(target.PreserveTables) == 0 {
		return d.runRestore(ctx, target, args, in)
	}

	// 보존할 테이블이 있으면 목차에서 그 항목을 빼고 되돌린다. 덤프에는 그 테이블이 들어 있다 —
	// 그대로 되돌리면 --clean 이 지금의 이력을 백업 시점 것으로 바꾼다.
	// 목차(-l)와 복원(-L)이 같은 아카이브를 두 번 읽어야 해서 파일로 받는다. 플랫폼 DB 는
	// 볼륨과 달리 수 MB 수준이다.
	archive, cleanup, err := spool(in)
	if err != nil {
		return err
	}
	defer cleanup()

	toc, err := d.listTOC(ctx, archive)
	if err != nil {
		return err
	}
	indexes, err := d.indexNames(ctx, target, target.PreserveTables)
	if err != nil {
		return fmt.Errorf("보존할 테이블의 인덱스 조회: %w", err)
	}
	list, err := os.CreateTemp("", "nullus-restore-*.list")
	if err != nil {
		return fmt.Errorf("복원 목록 파일: %w", err)
	}
	defer func() { _ = os.Remove(list.Name()) }()
	if _, err := list.WriteString(filterTOC(toc, target.PreserveTables, indexes)); err != nil {
		_ = list.Close()
		return fmt.Errorf("복원 목록 쓰기: %w", err)
	}
	if err := list.Close(); err != nil {
		return fmt.Errorf("복원 목록 쓰기: %w", err)
	}

	args = append(args, "-L", list.Name(), archive)
	return d.runRestore(ctx, target, args, nil)
}

func (d *PgDumper) runRestore(ctx context.Context, target port.DBTarget, args []string, in io.Reader) error {
	cmd := exec.CommandContext(ctx, d.restoreBin, args...)
	cmd.Env = append(cmd.Environ(), d.env(target)...)
	if in != nil {
		cmd.Stdin = in
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if hint := preservedTableFKHint(msg, target.PreserveTables); hint != "" {
			return fmt.Errorf("pg_restore 실패: %w (%s) — %s", err, msg, hint)
		}
		return fmt.Errorf("pg_restore 실패: %w (%s)", err, msg)
	}
	return nil
}

// preservedTableFKHint 는 보존하는 이력 테이블의 FK 가 --clean 을 막은 실패를 알아보고 조치를 말한다.
//
// 이력 테이블(backup_runs 등)이 되돌리는 테이블(organizations 등)을 FK 로 참조하면 pg_restore 가 그
// 테이블의 PK 를 지우는 순간 "cannot drop constraint … because other objects depend on it / DETAIL:
// constraint <fk> on table public.<이력 테이블> depends on …" 으로 죽는다. 2026-10-12 실환경 리허설에서
// 000091 의 backup_runs_org_id_fkey 가 그랬다(000092 가 거뒀다). 원문만으로는 무엇을 고쳐야 하는지
// 보이지 않는다 — 이력 테이블은 플랫폼 테이블을 참조하면 안 된다는 원칙과 조치를 덧붙인다.
func preservedTableFKHint(stderr string, preserved []string) string {
	if !strings.Contains(stderr, "cannot drop constraint") || !strings.Contains(stderr, "depends on") {
		return ""
	}
	var blocking []string
	for _, table := range preserved {
		if strings.Contains(stderr, "on table public."+table+" ") || strings.Contains(stderr, "on table "+table+" ") {
			blocking = append(blocking, table)
		}
	}
	if len(blocking) == 0 {
		return ""
	}
	return fmt.Sprintf("보존하는 백업 이력 테이블(%s)이 되돌리는 테이블을 FK 로 참조해 --clean 을 막습니다. "+
		"이력 테이블은 플랫폼 테이블을 FK 로 참조하면 안 됩니다 — 그 FK 를 거두는 마이그레이션을 적용한 뒤 다시 실행하세요(000092 참조)",
		strings.Join(blocking, ", "))
}

// spool 은 아카이브 스트림을 임시 파일로 받는다. 파일은 소유자만 읽는다(평문 DB 덤프다).
func spool(in io.Reader) (string, func(), error) {
	f, err := os.CreateTemp("", "nullus-restore-*.dump")
	if err != nil {
		return "", nil, fmt.Errorf("복원 임시 파일: %w", err)
	}
	cleanup := func() { _ = os.Remove(f.Name()) }
	if _, err := io.Copy(f, in); err != nil {
		_ = f.Close()
		cleanup()
		return "", nil, fmt.Errorf("복원 임시 파일 쓰기: %w", err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("복원 임시 파일 쓰기: %w", err)
	}
	return f.Name(), cleanup, nil
}

// listTOC 는 아카이브의 목차(pg_restore -l)를 읽는다.
func (d *PgDumper) listTOC(ctx context.Context, archive string) (string, error) {
	cmd := exec.CommandContext(ctx, d.restoreBin, "-l", archive)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("pg_restore 목차 읽기 실패: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}

// queryIndexNames 는 대상 DB 에서 그 테이블들의 인덱스 이름을 읽는다.
//
// 목차의 INDEX 항목은 인덱스 이름만 담는다. 이력 테이블의 인덱스는 마이그레이션이 이름을 정하므로
// 지금 DB 의 이름이 덤프 속 이름과 같다.
func (d *PgDumper) queryIndexNames(ctx context.Context, target port.DBTarget, tables []string) ([]string, error) {
	const q = `SELECT indexname FROM pg_indexes WHERE schemaname = 'public' AND tablename = ANY($1)`
	var rows pgx.Rows
	var err error
	if strings.TrimSpace(target.Host) == "" {
		if d.pool == nil {
			return nil, fmt.Errorf("대상 호스트도 DB 풀도 없습니다")
		}
		rows, err = d.pool.Query(ctx, q, tables)
	} else {
		conn, cerr := pgx.Connect(ctx, dsn(target))
		if cerr != nil {
			return nil, fmt.Errorf("%s 에 연결: %w", target.Database, cerr)
		}
		defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()
		rows, err = conn.Query(ctx, q, tables)
	}
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// filterTOC 는 목차에서 보존할 테이블에 딸린 항목을 뺀다. 나머지는 순서를 지킨다(-L 은 목록
// 순서대로 되돌린다).
//
// 항목 형식(pg_restore -l, PostgreSQL 18 실측): "<id>; <oid> <oid> <종류> <스키마> <이름…> <소유자>".
// 테이블·데이터·제약은 이름에 테이블이 들어 있고, INDEX 는 인덱스 이름만 있어 indexes 로 가른다.
func filterTOC(toc string, tables, indexes []string) string {
	if len(tables) == 0 {
		return toc
	}
	preserved := make(map[string]bool, len(tables))
	for _, t := range tables {
		preserved[t] = true
	}
	preservedIndex := make(map[string]bool, len(indexes))
	for _, i := range indexes {
		preservedIndex[i] = true
	}

	var b strings.Builder
	for _, line := range strings.SplitAfter(toc, "\n") {
		if !tocEntryTouches(line, preserved, preservedIndex) {
			b.WriteString(line)
		}
	}
	return b.String()
}

func tocEntryTouches(line string, tables, indexes map[string]bool) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, ";") {
		return false
	}
	_, rest, ok := strings.Cut(trimmed, "; ")
	if !ok {
		return false
	}
	f := strings.Fields(rest)
	if len(f) < 5 { // oid oid 종류 스키마 이름
		return false
	}
	f = f[2:] // 종류부터
	inPublic := func(i int) bool { return i < len(f) && f[i] == "public" }
	switch {
	case len(f) >= 3 && f[0] == "TABLE" && f[1] == "DATA":
		return inPublic(2) && len(f) > 3 && tables[f[3]]
	case len(f) >= 3 && f[0] == "FK" && f[1] == "CONSTRAINT":
		return inPublic(2) && len(f) > 3 && tables[f[3]]
	case f[0] == "TABLE", f[0] == "CONSTRAINT", f[0] == "TRIGGER", f[0] == "POLICY", f[0] == "RULE", f[0] == "DEFAULT":
		return inPublic(1) && tables[f[2]]
	case f[0] == "INDEX":
		return inPublic(1) && indexes[f[2]]
	case f[0] == "COMMENT", f[0] == "ACL":
		// "COMMENT public TABLE <테이블>" · "COMMENT public COLUMN <테이블>.<열>"
		if !inPublic(1) || len(f) < 4 {
			return false
		}
		name, _, _ := strings.Cut(f[3], ".")
		return (f[2] == "TABLE" || f[2] == "COLUMN") && tables[name]
	}
	return false
}

// SchemaState 는 **대상 DB** 의 golang-migrate schema_migrations 를 읽는다 (§6.2).
//
// 이 표는 플랫폼 DB 에만 있다 — Keycloak 은 Liquibase 를 써서 자기 스키마를
// 관리한다. 그래서 Keycloak 대상으로 부르면 "relation does not exist" 가
// 나는데, 그것이 옳다. 공용 풀로 대신 읽어 플랫폼의 버전을 돌려주면 **엉뚱한
// DB 의 답을 대상의 답인 것처럼** 쓰게 된다.
func (d *PgDumper) SchemaState(ctx context.Context, target port.DBTarget) (domain.SchemaState, error) {
	var st domain.SchemaState
	err := d.queryTarget(ctx, target,
		`SELECT version, dirty FROM schema_migrations LIMIT 1`, &st.Version, &st.Dirty)
	if err != nil {
		return domain.SchemaState{}, fmt.Errorf("schema_migrations 조회(%s): %w", target.Database, err)
	}
	return st, nil
}

var _ port.DBDumper = (*PgDumper)(nil)
