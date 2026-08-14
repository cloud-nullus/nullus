// Package jenkins 는 Jenkins 에 multibranch pipeline job 을 만든다.
//
// GitLab CI·GitHub Actions 와 근본적으로 다른 지점을 흡수하는 자리다. 그쪽은
// 파이프라인 정의를 푸시하면 자동으로 감지하지만, Jenkins 는 job 이 먼저
// 존재해야 한다 — Jenkinsfile 만 커밋해서는 아무 일도 일어나지 않는다.
//
// multibranch 를 쓰는 이유는 job 이 Jenkinsfile 을 저장소에서 스스로 찾기
// 때문이다. 스캐폴딩 결과와 자연스럽게 맞물리고, 브랜치가 늘어도 job 을 다시
// 만들 필요가 없다.
package jenkins

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

const (
	defaultTimeout       = 30 * time.Second
	maxErrorBodyReadSize = 4 << 10

	// giteaSCMSourceClass 는 gitea 플러그인이 제공하는 SCM 소스 타입이다.
	// 이 클래스가 없으면 job 은 만들어지지만 브랜치를 하나도 찾지 못한다.
	giteaSCMSourceClass = "org.jenkinsci.plugin.gitea.GiteaSCMSource"
)

// Client 는 port.CIJobProvisioner 의 Jenkins 구현체다.
type Client struct {
	baseURL    string
	user       string
	token      string
	httpClient *http.Client
}

// NewClient 는 Jenkins 클라이언트를 만든다.
//
// baseURL 은 컨트롤러 주소다 (예: http://jenkins.nullus.svc:8080).
// user/token 은 관리자 계정과 API 토큰(또는 비밀번호)이다.
func NewClient(baseURL, user, token string) *Client {
	// 쿠키 jar 를 둔다. Jenkins 는 CSRF crumb 을 세션에 묶어 검증하므로, crumb 을
	// 받은 요청과 그것을 쓰는 요청이 같은 세션이어야 한다 — 쿠키를 유지하지
	// 않으면 crumb 이 유효해도 "No valid crumb was included in the request" 로
	// 403 이 난다.
	//
	// jar 생성은 실패하지 않는 구성이지만(옵션 nil), 실패해도 클라이언트는
	// 동작해야 하므로 오류를 삼키고 jar 없이 진행한다.
	jar, _ := cookiejar.New(nil)

	return &Client{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		user:       strings.TrimSpace(user),
		token:      strings.TrimSpace(token),
		httpClient: &http.Client{Timeout: defaultTimeout, Jar: jar},
	}
}

// WithHTTPClient 는 타임아웃·전송 계층을 교체한다.
//
// 쿠키 jar 가 없으면 붙여 준다 — crumb 세션이 유지되지 않으면 모든 POST 가
// 403 으로 죽는다.
func (c *Client) WithHTTPClient(h *http.Client) *Client {
	if h != nil {
		if h.Jar == nil {
			jar, _ := cookiejar.New(nil)
			h.Jar = jar
		}
		c.httpClient = h
	}
	return c
}

// EnsureJob 은 multibranch pipeline job 을 만들거나 이미 있으면 그대로 둔다.
//
// 이미 있는 job 을 덮어쓰지 않는다 — 사용자가 Jenkins UI 에서 고친 설정(빌드
// 보존 기간, 추가 트리거 등)이 재프로비저닝마다 사라지면 안 된다.
func (c *Client) EnsureJob(ctx context.Context, spec port.CIJobSpec) (*port.CIJob, error) {
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		return nil, fmt.Errorf("jenkins job 이름이 필요합니다")
	}

	exists, err := c.jobExists(ctx, name)
	if err != nil {
		return nil, err
	}
	if exists {
		return &port.CIJob{Name: name, URL: c.jobURL(name)}, nil
	}

	configXML, err := multibranchConfigXML(spec)
	if err != nil {
		return nil, err
	}

	endpoint := "/createItem?name=" + url.QueryEscape(name)
	if err := c.post(ctx, endpoint, "application/xml", configXML); err != nil {
		return nil, fmt.Errorf("create jenkins job %q: %w", name, err)
	}
	return &port.CIJob{Name: name, URL: c.jobURL(name)}, nil
}

// DeleteJob 은 job 을 지운다. 이미 없으면 성공으로 본다 — 삭제의 목표는
// "없는 상태" 이고, 404 를 오류로 올리면 재시도가 영영 끝나지 않는다.
func (c *Client) DeleteJob(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("jenkins job 이름이 필요합니다")
	}
	err := c.post(ctx, "/job/"+url.PathEscape(name)+"/doDelete", "", nil)
	if err != nil && strings.Contains(err.Error(), "404") {
		return nil
	}
	return err
}

func (c *Client) jobExists(ctx context.Context, name string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/job/"+url.PathEscape(name)+"/api/json", nil)
	if err != nil {
		return false, err
	}
	c.applyAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("lookup jenkins job %q: %w", name, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode >= 300 {
		return false, fmt.Errorf("lookup jenkins job %q: %s", name, describeError(resp))
	}
	return true, nil
}

// post 는 CSRF crumb 을 붙여 POST 한다.
//
// Jenkins 는 기본으로 CSRF 보호가 켜져 있어 crumb 없이 POST 하면 403 이다.
// crumb 발급이 실패하면(보호가 꺼진 구성) 그대로 진행한다.
// get 은 JSON 응답을 읽는다. 404 는 오류가 아니라 found=false 다 —
// job 이 아직 없는 것은 정상 경로이기 때문이다.
func (c *Client) get(ctx context.Context, path string, out any) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return false, err
	}
	c.applyAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode >= 300 {
		return false, fmt.Errorf("jenkins GET %s: %s", path, describeError(resp))
	}
	if out == nil {
		return true, nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return false, fmt.Errorf("jenkins GET %s: 응답 해석 실패: %w", path, err)
	}
	return true, nil
}

func (c *Client) post(ctx context.Context, path, contentType string, body []byte) error {
	var reader io.Reader
	if body != nil {
		reader = strings.NewReader(string(body))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	c.applyAuth(req)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if field, value, ok := c.crumb(ctx); ok {
		req.Header.Set(field, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("jenkins POST %s: %s", path, describeError(resp))
	}
	return nil
}

// crumb 은 CSRF 토큰을 받아 온다. 실패하면 ok=false 다.
func (c *Client) crumb(ctx context.Context) (field, value string, ok bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/crumbIssuer/api/json", nil)
	if err != nil {
		return "", "", false
	}
	c.applyAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return "", "", false
	}

	var out struct {
		Crumb             string `json:"crumb"`
		CrumbRequestField string `json:"crumbRequestField"`
	}
	if err := decodeJSON(resp.Body, &out); err != nil {
		return "", "", false
	}
	if out.Crumb == "" {
		return "", "", false
	}
	fieldName := out.CrumbRequestField
	if fieldName == "" {
		fieldName = "Jenkins-Crumb"
	}
	return fieldName, out.Crumb, true
}

func (c *Client) applyAuth(req *http.Request) {
	if c.user != "" || c.token != "" {
		req.SetBasicAuth(c.user, c.token)
	}
}

func (c *Client) jobURL(name string) string {
	return c.baseURL + "/job/" + url.PathEscape(name) + "/"
}

// multibranchConfigXML 은 Gitea 소스를 보는 multibranch job 설정을 만든다.
//
// 문자열 템플릿 대신 구조체로 만들어 인코딩한다 — 앱 이름이나 주소에 &, < 가
// 섞이면 문자열 조합은 조용히 깨진 XML 을 만들고, Jenkins 는 그것을 "설정이
// 비어 있는 job" 으로 받아들여 브랜치를 하나도 찾지 못한다.
func multibranchConfigXML(spec port.CIJobSpec) ([]byte, error) {
	owner := strings.TrimSpace(spec.RepoOwner)
	repo := strings.TrimSpace(spec.RepoName)
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("jenkins job %q: 저장소 소유자와 이름이 필요합니다", spec.Name)
	}
	serverURL := strings.TrimSpace(spec.ServerURL)
	if serverURL == "" {
		return nil, fmt.Errorf("jenkins job %q: Gitea 서버 주소가 필요합니다", spec.Name)
	}
	pipelinePath := strings.TrimSpace(spec.PipelinePath)
	if pipelinePath == "" {
		pipelinePath = "Jenkinsfile"
	}

	cfg := multibranchProject{
		Plugin: "workflow-multibranch",
		Sources: sourcesBlock{
			Class: "jenkins.branch.MultiBranchProject$BranchSourceList",
			Data: branchSourceData{
				BranchSource: branchSource{
					Source: scmSource{
						Class:        giteaSCMSourceClass,
						ServerURL:    serverURL,
						RepoOwner:    owner,
						Repository:   repo,
						CredentialID: strings.TrimSpace(spec.CredentialID),
						// traits 는 비워 둘 수 없다. 없으면 Jenkins 가 job 생성
						// 중 NullPointerException("traits" is null)으로 500 을 낸다.
						//
						// strategyId=1 은 "브랜치를 모두 탐색"이다. PR 탐색은 넣지
						// 않는다 — 파이프라인은 기본 브랜치에서만 도는 것이 규약이고
						// (Jenkinsfile 의 when { branch 'main' }), PR trait 을 켜면
						// 포크마다 빌드가 돌아 자원을 예상 밖으로 먹는다.
						Traits: &traitsBlock{
							BranchDiscovery: &branchDiscoveryTrait{StrategyID: 1},
						},
					},
				},
			},
		},
		FactoryBlock: factoryBlock{
			Class:      "org.jenkinsci.plugins.workflow.multibranch.WorkflowBranchProjectFactory",
			ScriptPath: pipelinePath,
		},
	}

	raw, err := xml.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode jenkins job config for %q: %w", spec.Name, err)
	}
	return append([]byte(xml.Header), raw...), nil
}

type multibranchProject struct {
	XMLName      xml.Name     `xml:"org.jenkinsci.plugins.workflow.multibranch.WorkflowMultiBranchProject"`
	Plugin       string       `xml:"plugin,attr"`
	Sources      sourcesBlock `xml:"sources"`
	FactoryBlock factoryBlock `xml:"factory"`
}

type sourcesBlock struct {
	Class string           `xml:"class,attr"`
	Data  branchSourceData `xml:"data"`
}

type branchSourceData struct {
	BranchSource branchSource `xml:"jenkins.branch.BranchSource"`
}

type branchSource struct {
	Source scmSource `xml:"source"`
}

type scmSource struct {
	Class        string       `xml:"class,attr"`
	ServerURL    string       `xml:"serverUrl"`
	RepoOwner    string       `xml:"repoOwner"`
	Repository   string       `xml:"repository"`
	CredentialID string       `xml:"credentialsId,omitempty"`
	Traits       *traitsBlock `xml:"traits,omitempty"`
}

type traitsBlock struct {
	BranchDiscovery *branchDiscoveryTrait `xml:"org.jenkinsci.plugin.gitea.BranchDiscoveryTrait,omitempty"`
}

type branchDiscoveryTrait struct {
	StrategyID int `xml:"strategyId"`
}

type factoryBlock struct {
	Class      string `xml:"class,attr"`
	ScriptPath string `xml:"scriptPath"`
}

func describeError(resp *http.Response) string {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyReadSize))
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		return resp.Status
	}
	// Jenkins 는 오류를 HTML 로 돌려주기도 한다. 통째로 실으면 로그를 덮으므로
	// 앞부분만 남긴다.
	if len(msg) > 300 {
		msg = msg[:300] + "…"
	}
	return resp.Status + ": " + msg
}

func decodeJSON(r io.Reader, out any) error {
	return json.NewDecoder(r).Decode(out)
}

// ListBuilds 는 job 의 브랜치 빌드 이력을 읽는다.
//
// GitOps 경로에서는 플랫폼이 배포를 실행하지 않으므로 실행 기록이 여기에만
// 있다. 들이지 않으면 빌드가 성공해도 화면의 실행 통계가 0 으로 남는다.
func (c *Client) ListBuilds(ctx context.Context, jobName, branch string, limit int) ([]port.CIBuild, error) {
	job := strings.TrimSpace(jobName)
	if job == "" {
		return nil, fmt.Errorf("jenkins: job 이름이 필요합니다")
	}
	if limit <= 0 {
		limit = 20
	}

	// multibranch job 은 브랜치가 하위 job 이다. 브랜치를 빼면 폴더 자체를
	// 조회하게 되고 빌드가 하나도 없는 것처럼 보인다.
	path := "/job/" + url.PathEscape(job)
	if b := strings.TrimSpace(branch); b != "" {
		path += "/job/" + url.PathEscape(b)
	}
	path += fmt.Sprintf("/api/json?tree=builds[number,result,building,timestamp,duration]{,%d}", limit)

	var payload struct {
		Builds []struct {
			Number    int    `json:"number"`
			Result    string `json:"result"`
			Building  bool   `json:"building"`
			Timestamp int64  `json:"timestamp"`
			Duration  int64  `json:"duration"`
		} `json:"builds"`
	}
	found, err := c.get(ctx, path, &payload)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}

	out := make([]port.CIBuild, 0, len(payload.Builds))
	for _, b := range payload.Builds {
		out = append(out, port.CIBuild{
			Number:   b.Number,
			Result:   strings.TrimSpace(b.Result),
			Building: b.Building,
			// Jenkins 는 epoch 밀리초로 준다.
			StartedAt: time.UnixMilli(b.Timestamp),
			Duration:  time.Duration(b.Duration) * time.Millisecond,
		})
	}
	return out, nil
}
