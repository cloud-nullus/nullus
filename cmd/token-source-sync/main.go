package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloud-nullus/draft/internal/shared/secrets"
	"github.com/cloud-nullus/draft/internal/stack/domain"
	stackusecase "github.com/cloud-nullus/draft/internal/stack/usecase"
)

func main() {
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dsn == "" {
		dsn = "postgres://nullus:nullus_dev@localhost:5433/nullus?sslmode=disable"
	}
	openbaoAddr := strings.TrimSpace(os.Getenv("OPENBAO_ADDR"))
	openbaoToken := strings.TrimSpace(os.Getenv("OPENBAO_TOKEN"))
	if openbaoAddr == "" || openbaoToken == "" {
		log.Fatal("OPENBAO_ADDR and OPENBAO_TOKEN are required")
	}
	env := strings.TrimSpace(os.Getenv("TOKEN_SOURCE_ENV"))
	if env == "" {
		env = "dev"
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	secretRouter := secrets.NewRouter()
	secretRouter.Register("openbao", secrets.NewOpenBaoStore(openbaoAddr, openbaoToken))

	rows, err := pool.Query(ctx, `
		select id, org_id, namespace, config
		from stacks
		where deleted_at is null
		  and state = 'completed'
		  and coalesce(config->'authentication'->>'provider', '') = 'openbao'
		order by created_at asc`)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	var synced int
	for rows.Next() {
		var stackID, orgID, namespace string
		var rawConfig []byte
		if err := rows.Scan(&stackID, &orgID, &namespace, &rawConfig); err != nil {
			log.Fatal(err)
		}

		var cfg domain.StackConfig
		if err := json.Unmarshal(rawConfig, &cfg); err != nil {
			log.Fatalf("decode config for %s: %v", stackID, err)
		}
		stack := &domain.Stack{ID: stackID, OrgID: orgID, Namespace: namespace, Config: cfg}
		// 자격증명을 넘기지 않는다 — 이 도구는 저장된 구성만 보고 도는 백필이라
		// 사용자가 준 GitHub PAT 를 알 수 없다. 그 결과 GitHub 항목은 생성되지
		// 않으며, 등록은 설치(또는 재시도) 요청 경로에서만 이뤄진다.
		inputs := stackusecase.BuildStackTokenSourceInputs(stack, env, stackusecase.SourceControlCredentials{})
		for _, input := range inputs {
			if strings.TrimSpace(input.TokenValue) == "" {
				continue
			}
			if err := secretRouter.PutToken(ctx, input.SecretManager, input.Path, input.TokenValue); err != nil {
				log.Fatalf("sync %s: %v", input.Path, err)
			}
			synced++
		}
	}
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("synced %d token sources\n", synced)
}
