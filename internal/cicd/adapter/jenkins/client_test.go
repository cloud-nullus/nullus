package jenkins

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

func jobSpec() port.CIJobSpec {
	return port.CIJobSpec{
		Name:         "api",
		RepoOwner:    "nullus",
		RepoName:     "api",
		ServerURL:    "http://gitea-http.nullus.svc:3000",
		CredentialID: "nullus-gitea",
		PipelinePath: "Jenkinsfile",
	}
}

// 이미 있는 job 을 덮어쓰면 사용자가 Jenkins UI 에서 고친 설정이 사라진다.
func TestEnsureJob_ExistingIsNotRecreated(t *testing.T) {
	var createCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/job/api/api/json":
			w.WriteHeader(http.StatusOK)
		case strings.HasPrefix(r.URL.Path, "/createItem"):
			createCalls++
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	job, err := NewClient(srv.URL, "admin", "tok").EnsureJob(context.Background(), jobSpec())

	require.NoError(t, err)
	assert.Equal(t, "api", job.Name)
	assert.Zero(t, createCalls, "이미 있는 job 을 다시 만들면 UI 에서 고친 설정이 사라진다")
}

// Jenkins 는 CSRF 보호가 기본이라 crumb 없이 POST 하면 403 이다.
func TestEnsureJob_SendsCSRFCrumb(t *testing.T) {
	var gotCrumb string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/job/api/api/json":
			w.WriteHeader(http.StatusNotFound)
		case r.URL.Path == "/crumbIssuer/api/json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"crumb":"abc123","crumbRequestField":"Jenkins-Crumb"}`))
		case strings.HasPrefix(r.URL.Path, "/createItem"):
			gotCrumb = r.Header.Get("Jenkins-Crumb")
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, "admin", "tok").EnsureJob(context.Background(), jobSpec())

	require.NoError(t, err)
	assert.Equal(t, "abc123", gotCrumb)
}

// job 설정이 Gitea 소스를 가리켜야 브랜치를 찾는다. 클래스가 틀리면 job 은
// 만들어지지만 브랜치를 하나도 스캔하지 못한다.
func TestMultibranchConfigXML_PointsAtGiteaSource(t *testing.T) {
	raw, err := multibranchConfigXML(jobSpec())
	require.NoError(t, err)

	body := string(raw)
	assert.Contains(t, body, giteaSCMSourceClass)
	assert.Contains(t, body, "<repoOwner>nullus</repoOwner>")
	assert.Contains(t, body, "<repository>api</repository>")
	assert.Contains(t, body, "<scriptPath>Jenkinsfile</scriptPath>")
	assert.Contains(t, body, "<credentialsId>nullus-gitea</credentialsId>")
}

// 앱 이름이나 주소에 XML 특수문자가 섞여도 깨지지 않아야 한다.
// 문자열 조합이었다면 조용히 깨진 XML 이 되고 Jenkins 는 빈 설정으로 받아들인다.
func TestMultibranchConfigXML_EscapesSpecialCharacters(t *testing.T) {
	spec := jobSpec()
	spec.RepoName = "api&web"

	raw, err := multibranchConfigXML(spec)
	require.NoError(t, err)

	assert.NotContains(t, string(raw), "<repository>api&web</repository>")

	// 실제로 파싱되는 XML 인지 확인한다.
	var probe struct {
		XMLName xml.Name
	}
	require.NoError(t, xml.Unmarshal(raw, &probe))
}

func TestMultibranchConfigXML_RequiresOwnerAndServer(t *testing.T) {
	spec := jobSpec()
	spec.RepoOwner = ""
	_, err := multibranchConfigXML(spec)
	require.Error(t, err, "소유자 없이 job 을 만들면 브랜치를 찾지 못한 채 조용히 서 있는다")

	spec = jobSpec()
	spec.ServerURL = ""
	_, err = multibranchConfigXML(spec)
	require.Error(t, err)
}

// 삭제의 목표는 "없는 상태" 다.
func TestDeleteJob_MissingIsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/crumbIssuer/api/json" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	require.NoError(t, NewClient(srv.URL, "admin", "tok").DeleteJob(context.Background(), "api"))
}

// Jenkins 는 CSRF crumb 을 세션에 묶어 검증한다.
//
// crumb 을 받은 요청과 그것을 쓰는 요청이 같은 세션이어야 한다 — 쿠키를
// 유지하지 않으면 crumb 이 유효해도 "No valid crumb was included in the
// request" 로 403 이 난다. 실제로 job 생성이 이렇게 실패했다.
func TestClient_KeepsSessionAcrossCrumbAndPost(t *testing.T) {
	var crumbSession string
	var postCookie string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "crumbIssuer"):
			crumbSession = "sess-1"
			http.SetCookie(w, &http.Cookie{Name: "JSESSIONID", Value: crumbSession, Path: "/"})
			_, _ = w.Write([]byte(`{"crumbRequestField":"Jenkins-Crumb","crumb":"abc"}`))
		case r.Method == http.MethodPost:
			if c, err := r.Cookie("JSESSIONID"); err == nil {
				postCookie = c.Value
			}
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "admin", "pw")
	require.NoError(t, c.post(context.Background(), "/createItem?name=x", "application/xml", []byte("<x/>")))

	assert.Equal(t, crumbSession, postCookie,
		"crumb 을 받은 세션과 POST 세션이 같아야 한다")
}

// GiteaSCMSource 는 traits 없이 만들 수 없다.
//
// 비어 있으면 Jenkins 가 job 생성 중
// NullPointerException: Cannot invoke "java.util.Collection.iterator()"
// because "traits" is null
// 로 500 을 낸다 — 실제로 job 생성이 이렇게 실패했다.
//
// 브랜치 탐색 trait 이 있어야 multibranch 가 브랜치를 찾는다.
func TestMultibranchXML_IncludesBranchDiscoveryTraits(t *testing.T) {
	raw, err := multibranchConfigXML(port.CIJobSpec{
		Name:         "gj-demo-api",
		ServerURL:    "http://gitea.gj3.internal",
		RepoOwner:    "nullus",
		RepoName:     "gj-demo-api",
		CredentialID: "nullus-gitea",
		PipelinePath: "Jenkinsfile",
	})
	require.NoError(t, err)
	xml := string(raw)

	assert.Contains(t, xml, "<traits>",
		"traits 가 없으면 Jenkins 가 NPE 로 500 을 낸다")
	assert.Contains(t, xml, "BranchDiscoveryTrait",
		"브랜치 탐색 trait 이 없으면 브랜치를 하나도 찾지 못한다")
	assert.Contains(t, xml, "org.jenkinsci.plugin.gitea.GiteaSCMSource")
	assert.Contains(t, xml, "<scriptPath>Jenkinsfile</scriptPath>")
}

// GitOps 경로의 실행 기록은 Jenkins 가 갖고 있다.
// 들이지 않으면 빌드가 성공해도 화면의 실행 통계가 영원히 0 으로 남는다.
func TestClient_ListBuilds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 단계 정보는 별도 경로다. 플러그인이 없는 구성이 정상 경로이므로
		// 404 로 답해 단계 없이도 실행 기록이 남는지 함께 본다.
		if strings.Contains(r.URL.Path, "/wfapi/describe") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		assert.Contains(t, r.URL.Path, "/job/gj-demo-api/job/main/api/json")
		_, _ = w.Write([]byte(`{"builds":[
		  {"number":2,"result":null,"building":true,"timestamp":1786721700000,"duration":0},
		  {"number":1,"result":"SUCCESS","building":false,"timestamp":1786721616732,"duration":26494}
		]}`))
	}))
	defer srv.Close()

	builds, err := NewClient(srv.URL, "admin", "pw").
		ListBuilds(context.Background(), "gj-demo-api", "main", 20)
	require.NoError(t, err)
	require.Len(t, builds, 2)

	assert.Equal(t, 2, builds[0].Number)
	assert.True(t, builds[0].Building)
	assert.Empty(t, builds[0].Result, "실행 중인 빌드는 결과가 없다")

	assert.Equal(t, "SUCCESS", builds[1].Result)
	assert.Equal(t, 26494*time.Millisecond, builds[1].Duration)
	assert.Equal(t, int64(1786721616732), builds[1].StartedAt.UnixMilli())
	assert.Empty(t, builds[1].Stages, "플러그인이 없으면 단계 없이 실행 기록만 남는다")
}

// 단계 정보를 주는 구성에서는 정규화된 어휘로 옮긴다.
func TestClient_ListBuilds_NormalizesStages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/wfapi/describe") {
			_, _ = w.Write([]byte(`{"stages":[
			  {"name":"Build","status":"SUCCESS","startTimeMillis":1786721616732,"durationTimeMillis":20000},
			  {"name":"Deploy","status":"NOT_EXECUTED","startTimeMillis":0,"durationTimeMillis":0}
			]}`))
			return
		}
		_, _ = w.Write([]byte(`{"builds":[{"number":1,"result":"SUCCESS","building":false,"timestamp":1786721616732,"duration":26494}]}`))
	}))
	defer srv.Close()

	builds, err := NewClient(srv.URL, "admin", "pw").ListBuilds(context.Background(), "app", "main", 10)
	require.NoError(t, err)
	require.Len(t, builds, 1)
	require.Len(t, builds[0].Stages, 2)

	assert.Equal(t, port.CIStageSuccess, builds[0].Stages[0].Status)
	assert.Equal(t, port.CIStageSkipped, builds[0].Stages[1].Status,
		"NOT_EXECUTED 를 실패로 옮기면 안 된다")
}

// 폴더형 job 은 브랜치 경로가 없으면 빌드를 찾지 못한다.
func TestClient_ListBuilds_RequiresJobName(t *testing.T) {
	_, err := NewClient("http://x", "a", "b").ListBuilds(context.Background(), "  ", "main", 10)
	require.Error(t, err)
}
