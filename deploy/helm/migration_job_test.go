// DB 마이그레이션이 배포를 타고 실제로 적용되는지 본다.
//
// 이 배선이 없으면 아무도 실패를 보지 못한다 — helm 은 초록불로 끝나고 파드는
// Ready 가 되며, 새 코드가 아직 없는 컬럼을 읽을 때가 되어서야 500 이 난다.
// 실제로 nullus.io 가 그 상태였다: CD 는 helm upgrade 만 돌렸고, 차트의
// migration-job.yaml 은 "마이그레이션은 외부에서 처리한다"는 주석뿐인 빈
// 스텁이라, seed 마이그레이션으로만 들어가는 스택 템플릿이 배포 DB 에 하나도
// 없었고 ID/PW 로그인은 users.password_hash 가 없어 500 을 냈다.
package helm_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// 마이그레이션 SQL 이 이미지 안에서 놓이는 자리. Dockerfile 의
// `COPY db/migrations/ /etc/nullus/migrations/` 와 같은 값이어야 한다.
const migrationsDir = "/etc/nullus/migrations"

type envVar struct {
	Name      string `yaml:"name"`
	Value     string `yaml:"value"`
	ValueFrom *struct {
		SecretKeyRef *struct {
			Name string `yaml:"name"`
			Key  string `yaml:"key"`
		} `yaml:"secretKeyRef"`
	} `yaml:"valueFrom"`
}

type podContainer struct {
	Name    string   `yaml:"name"`
	Image   string   `yaml:"image"`
	Command []string `yaml:"command"`
	Args    []string `yaml:"args"`
	Env     []envVar `yaml:"env"`
}

// script 는 컨테이너가 실제로 실행하는 명령 전체다. command 와 args 중 어느
// 쪽에 본문을 실었는지에 관계없이 같은 문자열을 본다.
func (c podContainer) script() string {
	return strings.Join(append(append([]string{}, c.Command...), c.Args...), " ")
}

func (c podContainer) env(name string) (envVar, bool) {
	for _, e := range c.Env {
		if e.Name == name {
			return e, true
		}
	}
	return envVar{}, false
}

type workload struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name        string            `yaml:"name"`
		Labels      map[string]string `yaml:"labels"`
		Annotations map[string]string `yaml:"annotations"`
	} `yaml:"metadata"`
	Spec struct {
		BackoffLimit *int `yaml:"backoffLimit"`
		Template     struct {
			Spec struct {
				RestartPolicy  string         `yaml:"restartPolicy"`
				InitContainers []podContainer `yaml:"initContainers"`
				Containers     []podContainer `yaml:"containers"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

// decodeWorkloads 는 렌더 결과에서 파드를 만드는 오브젝트만 뜬다.
//
// 문서를 먼저 kind 만 보고 거른다. 서브차트(PostgreSQL/Keycloak)가 뱉는 다른
// 종류까지 같은 구조체로 밀어 넣으면 필드 타입이 어긋나 파싱에서 터진다.
func decodeWorkloads(t *testing.T, rendered string, kinds ...string) []workload {
	t.Helper()
	want := map[string]bool{}
	for _, k := range kinds {
		want[k] = true
	}

	dec := yaml.NewDecoder(bytes.NewReader([]byte(rendered)))
	var out []workload
	for {
		var node yaml.Node
		err := dec.Decode(&node)
		if err == io.EOF {
			return out
		}
		if err != nil {
			t.Fatalf("렌더 결과를 읽지 못했다: %v", err)
		}
		var head struct {
			Kind string `yaml:"kind"`
		}
		if err := node.Decode(&head); err != nil || !want[head.Kind] {
			continue
		}
		var w workload
		if err := node.Decode(&w); err != nil {
			t.Fatalf("%s 를 읽지 못했다: %v", head.Kind, err)
		}
		out = append(out, w)
	}
}

// migrationJob 은 렌더 결과에서 마이그레이션 Job 을 찾는다.
func migrationJob(t *testing.T, rendered string) workload {
	t.Helper()
	var found []workload
	for _, w := range decodeWorkloads(t, rendered, "Job") {
		if strings.Contains(w.Metadata.Name, "migrat") {
			found = append(found, w)
		}
	}
	if len(found) == 0 {
		t.Fatal("마이그레이션 Job 이 렌더되지 않았다 — 배포는 스키마를 그대로 두고 새 코드만 올린다")
	}
	if len(found) > 1 {
		t.Fatalf("마이그레이션 Job 이 %d 개다 — 동시에 돌면 서로의 schema_migrations 를 밟는다", len(found))
	}
	return found[0]
}

func migrationContainer(t *testing.T, job workload) podContainer {
	t.Helper()
	cs := job.Spec.Template.Spec.Containers
	if len(cs) != 1 {
		t.Fatalf("마이그레이션 Job 의 컨테이너가 %d 개다 — 하나여야 한다", len(cs))
	}
	return cs[0]
}

// apiContainer 는 api Deployment 의 본 컨테이너다. Job 이 여기에 맞춰져
// 있는지(같은 이미지, 같은 DB) 비교하는 기준이 된다.
func apiContainer(t *testing.T, rendered string) podContainer {
	t.Helper()
	for _, w := range decodeWorkloads(t, rendered, "Deployment") {
		if w.Metadata.Labels["app.kubernetes.io/component"] != "api" {
			continue
		}
		for _, c := range w.Spec.Template.Spec.Containers {
			if c.Name == "api" {
				return c
			}
		}
	}
	t.Fatal("api Deployment 를 찾지 못했다")
	return podContainer{}
}

// 설치와 업그레이드 **양쪽** 에서 돌아야 한다.
//
// pre-install 만 걸면 첫 설치 때 아직 없는 PostgreSQL 을 붙잡고 늘어지고,
// upgrade 에 안 걸면 이후 늘어나는 마이그레이션이 영영 적용되지 않는다.
// 배포 DB 가 손으로 migrate 를 돌린 시점에 멈춰 있던 원인이 후자다.
func TestMigrationJob_RunsOnInstallAndUpgrade(t *testing.T) {
	job := migrationJob(t, renderChart(t))

	hook := job.Metadata.Annotations["helm.sh/hook"]
	for _, want := range []string{"post-install", "pre-upgrade"} {
		if !strings.Contains(hook, want) {
			t.Errorf("helm.sh/hook 에 %q 가 없다 (현재: %q)", want, hook)
		}
	}

	// Job 의 파드 템플릿은 불변이라 같은 이름이 남아 있으면 두 번째 배포가
	// "field is immutable" 로 죽는다. 훅을 새로 만들기 전에 지워야 한다.
	policy := job.Metadata.Annotations["helm.sh/hook-delete-policy"]
	if !strings.Contains(policy, "before-hook-creation") {
		t.Errorf("helm.sh/hook-delete-policy 에 before-hook-creation 이 없다 (현재: %q) — 두 번째 배포가 이름 충돌로 실패한다", policy)
	}
}

// 새 코드보다 먼저 도는 것만으로는 부족하고, 실패했을 때 배포가 멈춰야 한다.
// DB 가 아직 안 떠서 한 번 실패한 것과 SQL 이 틀려서 실패한 것을 구분하려면
// 재시도가 있어야 한다.
func TestMigrationJob_RetriesWhileDatabaseWarmsUp(t *testing.T) {
	job := migrationJob(t, renderChart(t))

	if rp := job.Spec.Template.Spec.RestartPolicy; rp != "OnFailure" && rp != "Never" {
		t.Errorf("restartPolicy 가 %q 다 — Job 은 OnFailure 또는 Never 여야 한다", rp)
	}
	if job.Spec.BackoffLimit == nil {
		t.Fatal("backoffLimit 이 없다 — 기본값 6 회 재시도에 기대면 SQL 오류도 여섯 번 돈다")
	}
	if *job.Spec.BackoffLimit < 1 {
		t.Errorf("backoffLimit 이 %d 다 — DB 기동 지연 한 번에 배포가 통째로 실패한다", *job.Spec.BackoffLimit)
	}
}

// 마이그레이션은 **지금 올라가는 그 이미지** 의 SQL 이어야 한다.
// 태그가 따로 놀면 배포되는 코드와 다른 세대의 스키마가 적용된다.
func TestMigrationJob_UsesSameImageAsAPI(t *testing.T) {
	rendered := renderChart(t, "--set", "api.image.tag=test-tag")

	got := migrationContainer(t, migrationJob(t, rendered)).Image
	want := apiContainer(t, rendered).Image
	if got != want {
		t.Errorf("이미지가 api 와 다르다\n  Job: %s\n  api: %s", got, want)
	}
	if !strings.Contains(got, "test-tag") {
		t.Errorf("api.image.tag 를 따라가지 않는다: %s", got)
	}
}

// 이미지 안의 SQL 을 golang-migrate 로 up 한다.
func TestMigrationJob_AppliesMigrationsFromImage(t *testing.T) {
	script := migrationContainer(t, migrationJob(t, renderChart(t))).script()

	if !strings.Contains(script, migrationsDir) {
		t.Errorf("%s 를 읽지 않는다: %s", migrationsDir, script)
	}
	if !strings.Contains(script, "migrate ") {
		t.Errorf("golang-migrate 를 호출하지 않는다: %s", script)
	}
	if !strings.Contains(script, " up") {
		t.Errorf("up 방향으로 적용하지 않는다: %s", script)
	}
}

// Job 이 migrate 를 부르는 것과 이미지에 migrate 가 들어 있는 것은 별개다.
//
// deploy/csp/vm-cluster/runbook_csp.sh 의 Job 이 이미 그 상태였다 — api 이미지
// 안에서 migrate 를 실행하는데 Dockerfile 은 helm 과 kubectl 만 싣고 migrate 는
// 넣지 않아, 돌리면 "migrate: not found" 로 끝난다.
func TestAPIImage_ShipsMigrateBinary(t *testing.T) {
	root := filepath.Join("..", "..")
	raw, err := os.ReadFile(filepath.Join(root, "Dockerfile"))
	if err != nil {
		t.Fatalf("Dockerfile 읽기: %v", err)
	}
	dockerfile := string(raw)

	if !strings.Contains(dockerfile, "golang-migrate") {
		t.Error("Dockerfile 이 golang-migrate 를 싣지 않는다 — Job 은 migrate: not found 로 끝난다")
	}
	if !strings.Contains(dockerfile, migrationsDir) {
		t.Errorf("Dockerfile 이 SQL 을 %s 에 두지 않는다", migrationsDir)
	}
}

// 마이그레이션이 붙는 DB 와 API 가 붙는 DB 가 같아야 한다.
//
// 값을 따로 적어 두면 한쪽만 고쳤을 때 조용히 갈라진다 — 마이그레이션은
// 성공했는데 API 가 보는 DB 에는 반영이 없는 상태가 된다.
func TestMigrationJob_DatabaseWiringMatchesAPI(t *testing.T) {
	rendered := renderChart(t)
	job := migrationContainer(t, migrationJob(t, rendered))
	api := apiContainer(t, rendered)

	for _, name := range []string{
		"NULLUS_DATABASE_HOST",
		"NULLUS_DATABASE_PORT",
		"NULLUS_DATABASE_NAME",
		"NULLUS_DATABASE_USER",
		"NULLUS_DATABASE_SSLMODE",
	} {
		want, ok := api.env(name)
		if !ok {
			t.Fatalf("api 에 %s 가 없다 — 비교 기준이 사라졌다", name)
		}
		got, ok := job.env(name)
		if !ok {
			t.Errorf("마이그레이션 Job 에 %s 가 없다", name)
			continue
		}
		if got.Value != want.Value {
			t.Errorf("%s 가 api 와 다르다: Job=%q api=%q", name, got.Value, want.Value)
		}
	}

	wantPw, _ := api.env("NULLUS_DATABASE_PASSWORD")
	gotPw, ok := job.env("NULLUS_DATABASE_PASSWORD")
	if !ok {
		t.Fatal("마이그레이션 Job 에 NULLUS_DATABASE_PASSWORD 가 없다")
	}
	if gotPw.ValueFrom == nil || gotPw.ValueFrom.SecretKeyRef == nil {
		t.Fatal("비밀번호가 시크릿 참조가 아니다 — 매니페스트에 평문으로 남는다")
	}
	if wantPw.ValueFrom == nil || wantPw.ValueFrom.SecretKeyRef == nil {
		t.Fatal("api 의 비밀번호가 시크릿 참조가 아니다 — 비교 기준이 사라졌다")
	}
	if gotPw.ValueFrom.SecretKeyRef.Name != wantPw.ValueFrom.SecretKeyRef.Name ||
		gotPw.ValueFrom.SecretKeyRef.Key != wantPw.ValueFrom.SecretKeyRef.Key {
		t.Errorf("비밀번호 출처가 api 와 다르다: Job=%s/%s api=%s/%s",
			gotPw.ValueFrom.SecretKeyRef.Name, gotPw.ValueFrom.SecretKeyRef.Key,
			wantPw.ValueFrom.SecretKeyRef.Name, wantPw.ValueFrom.SecretKeyRef.Key)
	}
}

// 마이그레이션을 밖에서 돌리는 환경(airgap 등)은 훅을 끌 수 있어야 한다.
func TestMigrationJob_CanBeDisabled(t *testing.T) {
	rendered := renderChart(t, "--set", "migration.enabled=false")

	for _, w := range decodeWorkloads(t, rendered, "Job") {
		if strings.Contains(w.Metadata.Name, "migrat") {
			t.Errorf("migration.enabled=false 인데 %s 가 렌더됐다", w.Metadata.Name)
		}
	}
}
