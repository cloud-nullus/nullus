//go:build integration

// ServerVersion / SchemaState 가 **대상 DB** 를 실제로 보는지 검증한다.
//
// 둘 다 공용 풀(플랫폼 DB)로 읽고 target 을 무시하고 있었다. 그러면 Keycloak DB
// 처럼 다른 호스트·다른 버전의 대상에 대해 **엉뚱한 DB 의 답을 대상의 답인 것처럼**
// 쓰게 되고, 버전 호환성 검사와 매니페스트 기록이 조용히 틀어진다.
//
// 한 인스턴스에 두 database 를 두고(§1.2 통합 배치) 서로 다른 답이 나오는지로
// 확인한다 — 풀로 읽고 있으면 두 답이 같아져서 바로 드러난다.
//
// 실행: go test -tags integration ./internal/backup/adapter/postgres/
package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/cloud-nullus/draft/internal/backup/domain"
	"github.com/cloud-nullus/draft/internal/backup/port"
)

func setup(t *testing.T) (*PgDumper, port.DBTarget, port.DBTarget) {
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
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })

	conn, err := c.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	pool, err := pgxpool.New(ctx, conn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	// 플랫폼 DB 에만 schema_migrations 를 둔다 — 실제 배치와 같다.
	_, err = pool.Exec(ctx, `CREATE TABLE schema_migrations (version bigint primary key, dirty boolean not null)`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO schema_migrations VALUES (75, false)`)
	require.NoError(t, err)
	// Keycloak 은 Liquibase 를 쓴다 — schema_migrations 가 없다.
	_, err = pool.Exec(ctx, `CREATE DATABASE keycloak`)
	require.NoError(t, err)

	host, _ := c.Host(ctx)
	p5432, _ := c.MappedPort(ctx, "5432")
	mk := func(comp domain.Component, db string) port.DBTarget {
		return port.DBTarget{
			Component: comp, Host: host, Port: p5432.Int(),
			Database: db, User: "nullus", Password: "nullus_dev",
		}
	}
	return NewPgDumper(pool), mk(domain.ComponentPlatformDB, "nullus"), mk(domain.ComponentKeycloakDB, "keycloak")
}

func TestSchemaState_대상_DB_를_본다(t *testing.T) {
	d, platform, keycloak := setup(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	st, err := d.SchemaState(ctx, platform)
	require.NoError(t, err)
	assert.Equal(t, 75, st.Version)
	assert.False(t, st.Dirty)

	// Keycloak DB 에는 schema_migrations 가 없다. 풀로 읽고 있으면 여기서도
	// 75 가 나와 버린다 — 엉뚱한 DB 의 답을 대상의 답으로 쓰는 것이다.
	_, err = d.SchemaState(ctx, keycloak)
	require.Error(t, err, "대상에 없는 표를 플랫폼 DB 에서 대신 읽어오면 안 된다")
	assert.Contains(t, err.Error(), "keycloak")
}

func TestServerVersion_대상_DB_에_직접_붙는다(t *testing.T) {
	d, platform, keycloak := setup(t)
	ctx := context.Background()

	pv, err := d.ServerVersion(ctx, platform)
	require.NoError(t, err)
	assert.NotEmpty(t, pv)

	kv, err := d.ServerVersion(ctx, keycloak)
	require.NoError(t, err, "다른 database 에도 직접 붙을 수 있어야 한다")
	assert.Equal(t, pv, kv, "같은 인스턴스이므로 버전은 같다")
}

func TestServerVersion_대상에_닿지_못하면_오류다(t *testing.T) {
	// 조용히 풀로 떨어져 플랫폼 버전을 돌려주면, 호환성 검사가 거짓으로 통과한다.
	d, _, _ := setup(t)
	bad := port.DBTarget{
		Component: domain.ComponentKeycloakDB,
		Host:      "127.0.0.1", Port: 1, Database: "nope", User: "u", Password: "p",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, err := d.ServerVersion(ctx, bad)
	require.Error(t, err)
}
