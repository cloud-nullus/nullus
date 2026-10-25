package domain

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

// 업그레이드 번들 등록(설계 §12.4). 번들은 실클러스터 리허설을 거친 경로다 — 등록 화면이 생겨도
// 그 원칙은 그대로다. verified 로 넣거나 올리려면 운영자가 리허설을 마쳤다고 확인해야 하고,
// 확인 없이 들어오는 번들은 draft 에 머물러 후보가 되지 않는다.

var (
	ErrUpgradeBundleInvalid = errors.New("업그레이드 번들이 올바르지 않다")
	// ErrUpgradeBundleUnknown 은 상태와 무관하게 그 ID 의 번들이 없을 때다. ErrUpgradeBundleNotFound
	// 는 "실행할 수 있는(verified) 번들이 없다" 는 뜻이라 관리 화면에는 맞지 않는다.
	ErrUpgradeBundleUnknown           = errors.New("업그레이드 번들이 없다")
	ErrUpgradeBundleExists            = errors.New("같은 ID 또는 같은 경로(도구 · 출발 · 대상)의 업그레이드 번들이 이미 있다")
	ErrUpgradeBundleRehearsalRequired = errors.New("verified 번들은 실클러스터 리허설을 마쳤다고 확인해야 한다")
	ErrUpgradeBundleStatusTransition  = errors.New("허용되지 않는 번들 상태 변경이다")
)

// upgradeTools 는 업그레이드할 수 있는 도구다 — 상태 검사기(UpgradeHealthVerifier)가 있는 것만.
var upgradeTools = map[string]bool{UpgradeToolArgoCD: true}

// UpgradeToolSupported 는 그 도구의 업그레이드를 Nullus 가 실행할 수 있는지다.
func UpgradeToolSupported(tool string) bool {
	return upgradeTools[strings.ToLower(strings.TrimSpace(tool))]
}

var upgradeArchitectures = map[string]bool{"amd64": true, "arm64": true}

var (
	upgradeBundleIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,99}$`)
	cveIDPattern           = regexp.MustCompile(`^CVE-\d{4}-\d{4,}$`)
	upgradeRiskLevels      = map[string]bool{"low": true, "medium": true, "high": true}
)

type NewUpgradeBundleInput struct {
	// ID 를 비우면 <도구>-<출발 차트>-<대상 차트> 다.
	ID                     string
	Tool                   string
	ReleaseName            string
	ChartName              string
	RepoURL                string
	SourceChartVersion     string
	SourceAppVersion       string
	TargetChartVersion     string
	TargetAppVersion       string
	RiskLevel              string
	ExpectedDisruption     string
	ReleaseNotes           string
	RequiresBackup         bool
	SupportedArchitectures []string
	FixesCVEs              []string
	// Status 는 draft 또는 verified 다. 비우면 draft.
	Status string
	// RehearsalConfirmed 는 운영자가 "이 경로를 실클러스터에서 리허설했다" 고 확인한 것이다.
	// verified 는 이것 없이 만들 수 없다.
	RehearsalConfirmed bool
	Now                time.Time
}

// NewUpgradeBundle 은 등록 입력을 검사해 번들을 만든다. 버전은 읽을 수 있어야 하고 대상이 출발보다
// 높아야 한다 — 내려가는 경로나 같은 버전은 업그레이드가 아니다.
func NewUpgradeBundle(in NewUpgradeBundleInput) (*UpgradeBundle, error) {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", ErrUpgradeBundleInvalid, fmt.Sprintf(format, args...))
	}
	tool := strings.ToLower(strings.TrimSpace(in.Tool))
	if !UpgradeToolSupported(tool) {
		return nil, invalid("지원하지 않는 도구 %q", in.Tool)
	}
	release := strings.TrimSpace(in.ReleaseName)
	chart := strings.TrimSpace(in.ChartName)
	if release == "" || chart == "" {
		return nil, invalid("릴리스 이름과 차트 이름이 필요하다")
	}
	repo := strings.TrimSpace(in.RepoURL)
	if err := validateChartRepoURL(repo); err != nil {
		return nil, invalid("저장소 주소: %v", err)
	}
	source := strings.TrimSpace(in.SourceChartVersion)
	target := strings.TrimSpace(in.TargetChartVersion)
	if _, ok := parseChartVersion(source); !ok {
		return nil, invalid("출발 차트 버전 %q 을 읽을 수 없다", source)
	}
	if _, ok := parseChartVersion(target); !ok {
		return nil, invalid("대상 차트 버전 %q 을 읽을 수 없다", target)
	}
	if !chartVersionGreater(target, source) {
		return nil, invalid("대상 차트 버전 %s 은 출발 버전 %s 보다 높아야 한다", target, source)
	}
	risk := strings.ToLower(strings.TrimSpace(in.RiskLevel))
	if !upgradeRiskLevels[risk] {
		return nil, invalid("위험도는 low · medium · high 중 하나다")
	}
	archs, err := normalizeArchitectures(in.SupportedArchitectures)
	if err != nil {
		return nil, invalid("%v", err)
	}
	cves, err := normalizeCVEs(in.FixesCVEs)
	if err != nil {
		return nil, invalid("%v", err)
	}
	notes := strings.TrimSpace(in.ReleaseNotes)
	if strings.HasPrefix(notes, "http://") || strings.HasPrefix(notes, "https://") {
		if u, err := url.Parse(notes); err != nil || u.Host == "" {
			return nil, invalid("릴리스 노트 주소를 읽을 수 없다")
		}
	}
	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = fmt.Sprintf("%s-%s-%s", tool, source, target)
	}
	if !upgradeBundleIDPattern.MatchString(id) {
		return nil, invalid("ID 는 소문자 · 숫자 · . _ - 로 100자 이내여야 한다")
	}
	status := strings.ToLower(strings.TrimSpace(in.Status))
	switch status {
	case "":
		status = UpgradeBundleDraft
	case UpgradeBundleDraft:
	case UpgradeBundleVerified:
		if !in.RehearsalConfirmed {
			return nil, ErrUpgradeBundleRehearsalRequired
		}
	default:
		return nil, invalid("등록 때 상태는 draft 또는 verified 다")
	}
	return &UpgradeBundle{
		ID: id, Tool: tool, ReleaseName: release, ChartName: chart, RepoURL: repo,
		SourceChartVersion: source, SourceAppVersion: strings.TrimSpace(in.SourceAppVersion),
		TargetChartVersion: target, TargetAppVersion: strings.TrimSpace(in.TargetAppVersion),
		RiskLevel: risk, ExpectedDisruption: strings.TrimSpace(in.ExpectedDisruption), ReleaseNotes: notes,
		RequiresBackup: in.RequiresBackup, SupportedArchitectures: archs, Status: status, FixesCVEs: cves,
		CreatedAt: in.Now,
	}, nil
}

// ChangeStatus 는 번들의 상태를 바꾼다. verified 로 올리는 데는 리허설 확인이 필요하고, revoked 는
// 끝이다 — 거둔 경로를 되살리려면 다시 리허설해 새로 등록한다.
func (b *UpgradeBundle) ChangeStatus(to string, rehearsalConfirmed bool) error {
	to = strings.ToLower(strings.TrimSpace(to))
	from := b.Status
	if from == UpgradeBundleRevoked || to == from {
		return fmt.Errorf("%w: %s → %s", ErrUpgradeBundleStatusTransition, from, to)
	}
	switch to {
	case UpgradeBundleVerified:
		if from != UpgradeBundleDraft && from != UpgradeBundleDeprecated {
			return fmt.Errorf("%w: %s → %s", ErrUpgradeBundleStatusTransition, from, to)
		}
		if !rehearsalConfirmed {
			return ErrUpgradeBundleRehearsalRequired
		}
	case UpgradeBundleDeprecated:
		if from != UpgradeBundleVerified {
			return fmt.Errorf("%w: %s → %s", ErrUpgradeBundleStatusTransition, from, to)
		}
	case UpgradeBundleRevoked:
	default:
		return fmt.Errorf("%w: %s → %s", ErrUpgradeBundleStatusTransition, from, to)
	}
	b.Status = to
	return nil
}

func validateChartRepoURL(raw string) error {
	if raw == "" {
		return errors.New("비어 있다")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	switch u.Scheme {
	case "https", "http", "oci":
	default:
		return fmt.Errorf("스킴은 https · http · oci 중 하나여야 한다(%q)", u.Scheme)
	}
	if u.Host == "" {
		return errors.New("호스트가 없다")
	}
	return nil
}

func normalizeArchitectures(in []string) ([]string, error) {
	seen := map[string]bool{}
	out := []string{}
	for _, a := range in {
		a = strings.ToLower(strings.TrimSpace(a))
		if a == "" || seen[a] {
			continue
		}
		if !upgradeArchitectures[a] {
			return nil, fmt.Errorf("지원하지 않는 아키텍처 %q (amd64 · arm64)", a)
		}
		seen[a] = true
		out = append(out, a)
	}
	sort.Strings(out)
	return out, nil
}

func normalizeCVEs(in []string) ([]string, error) {
	seen := map[string]bool{}
	out := []string{}
	for _, id := range in {
		id = strings.ToUpper(strings.TrimSpace(id))
		if id == "" || seen[id] {
			continue
		}
		if !cveIDPattern.MatchString(id) {
			return nil, fmt.Errorf("CVE 식별자 형식이 아니다: %q (예: CVE-2025-12345)", id)
		}
		seen[id] = true
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
}
