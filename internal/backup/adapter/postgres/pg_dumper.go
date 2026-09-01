package postgres

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"

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

// ServerVersion 은 대상 DB 의 서버 버전을 돌려준다.
func (d *PgDumper) ServerVersion(ctx context.Context, target port.DBTarget) (string, error) {
	if d.pool == nil {
		return "", fmt.Errorf("DB 풀이 없습니다")
	}
	var v string
	if err := d.pool.QueryRow(ctx, "SHOW server_version").Scan(&v); err != nil {
		return "", fmt.Errorf("서버 버전 조회: %w", err)
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
	cMaj, cMin, err1 := majorMinor(client)
	sMaj, sMin, err2 := majorMinor(server)
	if err1 != nil || err2 != nil {
		return client, nil
	}
	if cMaj < sMaj || (cMaj == sMaj && cMin < sMin) {
		return "", fmt.Errorf(
			"pg_dump 버전(%s)이 서버 버전(%s)보다 낮습니다. 이 조합은 백업을 조용히 실패시킵니다", client, server)
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

// SchemaState 는 golang-migrate 의 schema_migrations 를 읽는다 (§6.2).
func (d *PgDumper) SchemaState(ctx context.Context, target port.DBTarget) (domain.SchemaState, error) {
	if d.pool == nil {
		return domain.SchemaState{}, fmt.Errorf("DB 풀이 없습니다")
	}
	var st domain.SchemaState
	err := d.pool.QueryRow(ctx, `SELECT version, dirty FROM schema_migrations LIMIT 1`).Scan(&st.Version, &st.Dirty)
	if err != nil {
		return domain.SchemaState{}, fmt.Errorf("schema_migrations 조회: %w", err)
	}
	return st, nil
}

var _ port.DBDumper = (*PgDumper)(nil)
