package nullusclient

import (
	"context"
	"fmt"
	"net/http"

	"github.com/blang/semver/v4"
)

// MinServerVersion 은 이 클라이언트가 지원하는 최소 서버 버전이다 (백로그 S-4).
// 서버 API 에 breaking change 가 들어갈 때만 올린다 — 값의 출처는
// cmd/api/main.go /health 의 version 필드.
const MinServerVersion = "0.1.0-alpha"

// ServerInfo 는 GET /health 응답이다. /health 는 인증 없이 열려 있어
// 로그인 전 버전 검사에도 쓸 수 있다.
type ServerInfo struct {
	Status  string `json:"status"`
	DB      string `json:"db"`
	Version string `json:"version"`
}

// ServerInfo 는 서버 상태·버전을 조회한다.
func (c *Client) ServerInfo(ctx context.Context) (ServerInfo, error) {
	var info ServerInfo
	if err := c.Do(ctx, http.MethodGet, "/health", nil, &info); err != nil {
		return ServerInfo{}, err
	}
	return info, nil
}

// VersionSkew 는 클라이언트-서버 버전 스큐 판정 결과다. 경고를 낼지(stderr)
// 중단할지(Automation 계약 §1, exit 5)는 호출측이 정한다.
type VersionSkew struct {
	ServerVersion string // 서버가 보고한 원문 — 경고 문구용
	MinSupported  string // = MinServerVersion
	Compatible    bool   // 서버 버전 >= 최소 호환 버전
}

// CheckVersionSkew 는 서버 버전을 조회해 최소 호환 버전과 비교한다.
// 서버 도달 실패는 *APIError, 버전 파싱 불능은 일반 오류로 돌아온다.
func (c *Client) CheckVersionSkew(ctx context.Context) (VersionSkew, error) {
	info, err := c.ServerInfo(ctx)
	if err != nil {
		return VersionSkew{}, err
	}
	ok, err := versionCompatible(info.Version, MinServerVersion)
	if err != nil {
		return VersionSkew{}, fmt.Errorf("서버 버전 %q 판정 불능: %w", info.Version, err)
	}
	return VersionSkew{
		ServerVersion: info.Version,
		MinSupported:  MinServerVersion,
		Compatible:    ok,
	}, nil
}

// versionCompatible 은 server >= min 인지 semver 로 판정한다. "v" 접두어와
// 짧은 버전("0.1")은 관대하게 받는다.
func versionCompatible(server, min string) (bool, error) {
	sv, err := semver.ParseTolerant(server)
	if err != nil {
		return false, err
	}
	mv, err := semver.ParseTolerant(min)
	if err != nil {
		return false, fmt.Errorf("최소 호환 버전 %q 자체가 잘못됐다: %w", min, err)
	}
	return sv.GTE(mv), nil
}
