package postgres

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/url"
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
}

func NewPgDumper(pool *pgxpool.Pool) *PgDumper {
	return &PgDumper{pool: pool, dumpBin: "pg_dump", restoreBin: "pg_restore", psqlBin: "psql"}
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

	cmd := exec.CommandContext(ctx, d.restoreBin, args...)
	cmd.Env = append(cmd.Environ(), d.env(target)...)
	cmd.Stdin = in
	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_restore 실패: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}
	return nil
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
