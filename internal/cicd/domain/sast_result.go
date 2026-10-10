package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// QualityGateStatus 는 SonarQube 가 낸 Quality Gate 판정이다.
type QualityGateStatus string

const (
	QualityGateOK    QualityGateStatus = "OK"
	QualityGateError QualityGateStatus = "ERROR"
	// QualityGateNone 은 게이트가 걸리지 않은 프로젝트다.
	QualityGateNone QualityGateStatus = "NONE"
)

// SASTResult 는 실행 하나의 소스 정적 분석 결과 요약이다.
//
// 이미지 스캔처럼 요약·판정·위치만 둔다 — 이슈 목록 원본은 SonarQube 가 갖는다
// (이미지 스캔 설계 §8 과 같은 범위). DashboardURL 로 SonarQube 화면을 연다.
type SASTResult struct {
	ID           string `json:"id"`
	PipelineID   string `json:"pipeline_id"`
	DeploymentID string `json:"deployment_id,omitempty"`

	ProjectKey string `json:"project_key,omitempty"`
	AnalysisID string `json:"analysis_id,omitempty"`

	// QualityGateStatus 는 SonarQube 의 판정이다. 분석하지 못했으면 비어 있다.
	QualityGateStatus QualityGateStatus `json:"quality_gate_status,omitempty"`
	// GateResult 는 파이프라인 관점의 판정이다 — 같은 Quality Gate 실패도 스택 정책에 따라
	// 차단·경고로 갈린다.
	GateResult GateResult      `json:"gate_result"`
	Conditions []SASTCondition `json:"conditions,omitempty"`
	// Metrics 는 분석했을 때만 있다. nil 은 "모름" 이다 — 0 으로 채우면 "문제 0건" 으로 읽힌다.
	Metrics *SASTMetrics `json:"metrics,omitempty"`

	DashboardURL string    `json:"dashboard_url,omitempty"`
	AnalyzedAt   time.Time `json:"analyzed_at"`
}

// SASTResultID 는 실행 하나에 대응하는 분석 결과 ID 다. 같은 실행을 다시 동기화해도 기록이 늘지 않는다.
func SASTResultID(deploymentID string) string {
	return "sast_" + deploymentID
}

// SASTDashboardURL 은 사람이 열 SonarQube 프로젝트 화면 주소다.
//
// 스캐너가 리포트에 남기는 주소는 스캐너가 붙은 서버 주소(클러스터 내 서비스)라 브라우저에서
// 열리지 않는다 — 서버의 공개 주소 설정(sonar.core.serverBaseURL)이 있어도 그렇다(실측). 스택의
// 공개 주소(webBase)를 알면 그것으로 만들고, 모르면 리포트 주소를 쓰되 클러스터 안 주소는 버린다.
// 죽은 링크보다 없는 편이 낫다.
func SASTDashboardURL(webBase, projectKey, reported string) string {
	webBase = strings.TrimRight(strings.TrimSpace(webBase), "/")
	projectKey = strings.TrimSpace(projectKey)
	if webBase != "" && projectKey != "" {
		return webBase + "/dashboard?id=" + url.QueryEscape(projectKey)
	}
	reported = strings.TrimSpace(reported)
	u, err := url.Parse(reported)
	if reported == "" || err != nil || u.Hostname() == "" {
		return ""
	}
	host := u.Hostname()
	if !strings.Contains(host, ".") || strings.HasSuffix(host, ".svc") || strings.HasSuffix(host, ".svc.cluster.local") {
		return ""
	}
	return reported
}

// SASTCondition 은 Quality Gate 조건 하나의 평가 결과다.
type SASTCondition struct {
	Metric     string `json:"metric"`
	Comparator string `json:"comparator,omitempty"`
	Threshold  string `json:"threshold,omitempty"`
	Actual     string `json:"actual,omitempty"`
	// Status 는 OK 또는 ERROR 다.
	Status string `json:"status"`
}

// SASTMetrics 는 프로젝트 전체 지표다. 응답에 없는 지표는 nil 이다.
type SASTMetrics struct {
	Bugs                   *int     `json:"bugs,omitempty"`
	Vulnerabilities        *int     `json:"vulnerabilities,omitempty"`
	CodeSmells             *int     `json:"code_smells,omitempty"`
	SecurityHotspots       *int     `json:"security_hotspots,omitempty"`
	Coverage               *float64 `json:"coverage,omitempty"`
	DuplicatedLinesDensity *float64 `json:"duplicated_lines_density,omitempty"`
	Ncloc                  *int     `json:"ncloc,omitempty"`
}

// SASTReport 는 파이프라인 SAST 잡이 남긴 리포트를 읽은 것이다.
type SASTReport struct {
	// ScannerExitCode 는 sonar-scanner 의 종료 코드다. 0 통과, 3 Quality Gate 실패,
	// 그 밖은 분석 실패(서버 장애·인증 실패)다(실측).
	ScannerExitCode   int
	ProjectKey        string
	AnalysisID        string
	DashboardURL      string
	QualityGateStatus QualityGateStatus
	Conditions        []SASTCondition
	Metrics           *SASTMetrics
}

// sonar-scanner 종료 코드 — 스캐폴딩의 분석 스크립트와 같은 값이다.
const (
	sastExitPassed     = 0
	sastExitGateFailed = 3
)

type sastReportJSON struct {
	ScannerExitCode *int   `json:"scanner_exit_code"`
	ProjectKey      string `json:"project_key"`
	AnalysisID      string `json:"analysis_id"`
	DashboardURL    string `json:"dashboard_url"`
	QualityGate     *struct {
		ProjectStatus struct {
			Status     string `json:"status"`
			Conditions []struct {
				Status         string `json:"status"`
				MetricKey      string `json:"metricKey"`
				Comparator     string `json:"comparator"`
				ErrorThreshold string `json:"errorThreshold"`
				ActualValue    string `json:"actualValue"`
			} `json:"conditions"`
		} `json:"projectStatus"`
	} `json:"quality_gate"`
	Measures *struct {
		Component struct {
			Measures []struct {
				Metric string `json:"metric"`
				Value  string `json:"value"`
			} `json:"measures"`
		} `json:"component"`
	} `json:"measures"`
}

// ParseSASTReport 는 SAST 잡이 남긴 리포트를 읽는다.
//
// quality_gate·measures 는 SonarQube 응답을 그대로 담은 것이다. 스캐너 종료 코드가 없으면
// 판정의 근거가 없어 형식 오류로 본다.
func ParseSASTReport(raw []byte) (*SASTReport, error) {
	var doc sastReportJSON
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("SAST 리포트 형식 오류: %w", err)
	}
	if doc.ScannerExitCode == nil {
		return nil, errors.New("SAST 리포트에 scanner_exit_code 가 없다")
	}

	r := &SASTReport{
		ScannerExitCode: *doc.ScannerExitCode,
		ProjectKey:      strings.TrimSpace(doc.ProjectKey),
		AnalysisID:      strings.TrimSpace(doc.AnalysisID),
		DashboardURL:    strings.TrimSpace(doc.DashboardURL),
	}
	if qg := doc.QualityGate; qg != nil {
		r.QualityGateStatus = QualityGateStatus(strings.ToUpper(strings.TrimSpace(qg.ProjectStatus.Status)))
		for _, c := range qg.ProjectStatus.Conditions {
			r.Conditions = append(r.Conditions, SASTCondition{
				Metric:     c.MetricKey,
				Comparator: c.Comparator,
				Threshold:  c.ErrorThreshold,
				Actual:     c.ActualValue,
				Status:     c.Status,
			})
		}
	}
	if m := doc.Measures; m != nil {
		metrics := &SASTMetrics{}
		for _, mv := range m.Component.Measures {
			switch mv.Metric {
			case "bugs":
				metrics.Bugs = parseIntMeasure(mv.Value)
			case "vulnerabilities":
				metrics.Vulnerabilities = parseIntMeasure(mv.Value)
			case "code_smells":
				metrics.CodeSmells = parseIntMeasure(mv.Value)
			case "security_hotspots":
				metrics.SecurityHotspots = parseIntMeasure(mv.Value)
			case "coverage":
				metrics.Coverage = parseFloatMeasure(mv.Value)
			case "duplicated_lines_density":
				metrics.DuplicatedLinesDensity = parseFloatMeasure(mv.Value)
			case "ncloc":
				metrics.Ncloc = parseIntMeasure(mv.Value)
			}
		}
		r.Metrics = metrics
	}
	return r, nil
}

func parseIntMeasure(v string) *int {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return nil
	}
	return &n
}

func parseFloatMeasure(v string) *float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil {
		return nil
	}
	return &f
}

// SASTGateFromStageAndReport 는 분석 단계 결과와 리포트로 판정한다.
//
// 단계 결과에는 스택 정책이 이미 반영돼 있다 — 경고 정책이면 게이트에 걸려도 성공, 장애 허용
// 정책이면 분석을 못 해도 성공이다. 그래서 단계 결과만으로는 셋을 가를 수 없고, 리포트의
// 스캐너 종료 코드가 "게이트에 걸렸다" 와 "분석을 못 했다" 를 가른다.
//
// 리포트가 없으면 판정하지 않는다. 리포트를 남기기 전에 만든 파이프라인의 실패를 차단으로
// 적으면, 분석기 장애가 "보안 문제로 막힌 배포" 로 보인다.
func SASTGateFromStageAndReport(stageStatus string, r *SASTReport) (GateResult, bool) {
	fromStage, ok := GateResultFromStageStatus(stageStatus)
	if !ok || r == nil {
		return "", false
	}
	switch r.ScannerExitCode {
	case sastExitPassed:
		return GateResultPass, true
	case sastExitGateFailed:
		if fromStage == GateResultPass {
			// 경고 정책이 통과시켰다. 배포는 나갔으므로 차단으로 세지 않는다.
			return GateResultWarn, true
		}
		return GateResultBlock, true
	default:
		// 분석하지 못했다. 장애 허용 정책이 통과시켰어도 분석하지 않은 코드를 통과로 적지 않는다.
		return GateResultError, true
	}
}
