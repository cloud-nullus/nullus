package gitlab

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// GitLab 은 기본 설정으로 webhook 이 사설망 주소로 가는 것을 막는다
// (allow_local_requests_from_web_hooks_and_services=false). 클러스터 안의 Argo CD 서버로
// 커밋을 알리려면 그 호스트가 허용 목록(outbound_local_requests_allowlist)에 있어야 한다.
// 전체를 풀면 모든 프로젝트의 webhook 이 클러스터 안을 찌를 수 있으므로, 대상 호스트
// 하나만 더한다.

const applicationSettingsPath = "/api/v4/application/settings"

// EnsureWebhook 은 프로젝트에 push webhook 을 건다(멱등).
//
// 대상이 클러스터 안 주소면 먼저 그 호스트를 허용 목록에 더한다. GitLab 은 webhook 을
// 만들 때 주소를 검사하므로 순서가 바뀌면 생성이 "local network" 로 거부된다. 허용
// 목록을 바꾸려면 관리자 토큰이어야 한다(플랫폼 토큰은 root 의 자동화 PAT 다).
func (c *Client) EnsureWebhook(ctx context.Context, projectID, targetURL, secret string) error {
	projectID = strings.TrimSpace(projectID)
	targetURL = strings.TrimSpace(targetURL)
	if projectID == "" {
		return fmt.Errorf("project id is required")
	}
	if targetURL == "" {
		return fmt.Errorf("webhook 대상 주소가 필요합니다 (project=%s)", projectID)
	}
	target, err := url.Parse(targetURL)
	if err != nil || target.Hostname() == "" {
		return fmt.Errorf("webhook 대상 주소가 올바르지 않습니다: %q", targetURL)
	}

	hooksPath := fmt.Sprintf("/api/v4/projects/%s/hooks", url.PathEscape(projectID))
	var existing []struct {
		URL string `json:"url"`
	}
	// 기본 페이지(20개)만 보면 hook 이 많은 저장소에서 이미 건 것을 놓쳐 하나 더 건다.
	if _, err := c.get(ctx, hooksPath+"?per_page=100", &existing); err != nil {
		return fmt.Errorf("list webhooks of project %s: %w", projectID, err)
	}
	for _, hook := range existing {
		if strings.EqualFold(strings.TrimSpace(hook.URL), targetURL) {
			return nil
		}
	}

	clusterTarget := isClusterServiceHost(target.Hostname())
	if clusterTarget {
		if err := c.allowLocalRequestsTo(ctx, target); err != nil {
			return err
		}
	}

	body := map[string]any{
		"url":         targetURL,
		"push_events": true,
	}
	if strings.TrimSpace(secret) != "" {
		body["token"] = secret
	}
	if err := c.post(ctx, hooksPath, body, nil); err != nil {
		// 허용 목록에 넣은 클러스터 주소가 422 면 GitLab 이 아직 옛 설정으로 검사한 것이다
		// (설정은 프로세스마다 최대 1분 캐시된다). 2026-10-19 실측에서 목록을 바꾼 직후의
		// 생성이 "Invalid url given" 으로 거부되고, 1분 뒤 같은 주소는 만들어졌다.
		// 다른 422(hook 개수 한도 등)는 기다린다고 풀리지 않으니 본문으로 가린다.
		var apiErr *apiError
		if clusterTarget && errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusUnprocessableEntity &&
			strings.Contains(apiErr.Body, "Invalid url") {
			return fmt.Errorf("create webhook on project %s: %w (%s)", projectID, port.ErrWebhookTargetPending, apiErr.Body)
		}
		return fmt.Errorf("create webhook on project %s: %w", projectID, err)
	}
	return nil
}

// allowLocalRequestsTo 는 대상의 호스트:포트를 GitLab 의 사설망 허용 목록에 더한다.
//
// 포트까지 적는다 — 호스트만 적으면 그 호스트의 모든 포트가 열린다. 같은 호스트가 포트 없이
// 이미 있으면 그것으로 충분하니 그대로 둔다.
//
// GitLab 은 모르는 설정 인자를 조용히 무시한다. 바꿨다고 믿고 넘어가면 webhook 생성이
// 막힌 이유를 알 수 없으므로, 다시 읽어 반영됐는지 확인한다.
func (c *Client) allowLocalRequestsTo(ctx context.Context, target *url.URL) error {
	host := target.Hostname()
	entry := hostWithPort(target)
	entries, err := c.localRequestsAllowlist(ctx)
	if err != nil {
		return err
	}
	if slices.Contains(entries, entry) || slices.Contains(entries, host) {
		return nil
	}
	// 원문으로 보낸다. GitLab 17.7 은 배열 인자(outbound_local_requests_whitelist)를 받지
	// 않고 "받을 인자가 없다" 는 400 을 낸다 — 2026-10-19 실측.
	updated := append(slices.Clone(entries), entry)
	if err := c.put(ctx, applicationSettingsPath, map[string]any{
		"outbound_local_requests_allowlist_raw": strings.Join(updated, "\n"),
	}, nil); err != nil {
		return fmt.Errorf("webhook 허용 목록에 %s 를 더하지 못했습니다: %w", entry, briefSettingsError(err))
	}

	entries, err = c.localRequestsAllowlist(ctx)
	if err != nil {
		return err
	}
	if !slices.Contains(entries, entry) {
		return fmt.Errorf("webhook 허용 목록 변경이 GitLab 에 반영되지 않았습니다 (%s)", entry)
	}
	return nil
}

// localRequestsAllowlist 는 지금의 허용 목록을 읽는다.
//
// 응답에 그 항목이 아예 없으면(다른 버전 · 권한) 빈 목록으로 보지 않는다 — 그렇게 보고 쓰면
// 관리자가 넣어 둔 항목을 통째로 덮어쓴다.
func (c *Client) localRequestsAllowlist(ctx context.Context) ([]string, error) {
	var settings struct {
		AllowlistRaw *string `json:"outbound_local_requests_allowlist_raw"`
	}
	if _, err := c.get(ctx, applicationSettingsPath, &settings); err != nil {
		return nil, fmt.Errorf("webhook 허용 목록을 읽지 못했습니다: %w", briefSettingsError(err))
	}
	if settings.AllowlistRaw == nil {
		return nil, fmt.Errorf("GitLab 설정 응답에 webhook 허용 목록(outbound_local_requests_allowlist_raw)이 없습니다")
	}
	return splitAllowlist(*settings.AllowlistRaw), nil
}

// hostWithPort 는 허용 목록 항목(호스트:포트)이다. 포트가 없으면 스킴의 기본 포트를 쓴다.
func hostWithPort(target *url.URL) string {
	if target.Port() != "" {
		return target.Host
	}
	if strings.EqualFold(target.Scheme, "https") {
		return target.Hostname() + ":443"
	}
	return target.Hostname() + ":80"
}

// splitAllowlist 는 GitLab 의 허용 목록 원문을 항목으로 나눈다. 화면에서는 줄바꿈 ·
// 쉼표 · 세미콜론을 모두 구분자로 받는다.
func splitAllowlist(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\n' || r == '\r' || r == ',' || r == ';' || r == ' ' || r == '\t'
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

// isClusterServiceHost 는 쿠버네티스 서비스 DNS 이름인지 본다. 이런 주소는 사설
// ClusterIP 로 풀려 GitLab 이 기본으로 막는다.
func isClusterServiceHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	return strings.HasSuffix(host, ".svc") || strings.HasSuffix(host, ".svc.cluster.local")
}

// maxSettingsErrorBody 는 설정 API 오류 본문을 경고에 싣는 길이다. GitLab 의 400 은
// 받을 수 있는 설정 이름 수백 개를 늘어놓는다.
const maxSettingsErrorBody = 200

// briefSettingsError 는 설정 API 오류를 화면 경고에 실을 만큼 줄이고, 권한 문제면 그렇게 말한다.
func briefSettingsError(err error) error {
	var apiErr *apiError
	if !errors.As(err, &apiErr) {
		return err
	}
	if apiErr.StatusCode == http.StatusUnauthorized || apiErr.StatusCode == http.StatusForbidden {
		return fmt.Errorf("gitlab api status=%d — 관리자 권한이 있는 토큰이어야 합니다", apiErr.StatusCode)
	}
	body := []rune(apiErr.Body)
	if len(body) > maxSettingsErrorBody {
		body = append(body[:maxSettingsErrorBody], '…')
	}
	return fmt.Errorf("gitlab api status=%d body=%s", apiErr.StatusCode, string(body))
}
