package jenkins

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
