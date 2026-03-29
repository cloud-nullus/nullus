package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	obshandler "github.com/cloud-nullus/draft/internal/observability/adapter/handler"
	"github.com/cloud-nullus/draft/internal/observability/domain"
	"github.com/cloud-nullus/draft/internal/observability/usecase"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockAlertRuleRepository struct {
	rules         map[string]*domain.AlertRule
	updateCalls   int
	deleteCalls   int
	createCalls   int
	listCalls     int
	getByIDCalls  int
	updateErrByID map[string]error
	deleteErrByID map[string]error
}

func newMockAlertRuleRepository(seed ...*domain.AlertRule) *mockAlertRuleRepository {
	rules := make(map[string]*domain.AlertRule, len(seed))
	for _, rule := range seed {
		copied := *rule
		rules[rule.ID] = &copied
	}

	return &mockAlertRuleRepository{
		rules:         rules,
		updateErrByID: map[string]error{},
		deleteErrByID: map[string]error{},
	}
}

func (m *mockAlertRuleRepository) Create(_ context.Context, rule *domain.AlertRule) error {
	m.createCalls++
	copied := *rule
	m.rules[rule.ID] = &copied
	return nil
}

func (m *mockAlertRuleRepository) GetByID(_ context.Context, id string) (*domain.AlertRule, error) {
	m.getByIDCalls++
	rule, ok := m.rules[id]
	if !ok {
		return nil, domain.ErrAlertRuleNotFound
	}
	copied := *rule
	return &copied, nil
}

func (m *mockAlertRuleRepository) List(_ context.Context) ([]*domain.AlertRule, error) {
	m.listCalls++
	items := make([]*domain.AlertRule, 0, len(m.rules))
	for _, rule := range m.rules {
		copied := *rule
		items = append(items, &copied)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})
	return items, nil
}

func (m *mockAlertRuleRepository) Update(_ context.Context, rule *domain.AlertRule) error {
	m.updateCalls++
	if err, ok := m.updateErrByID[rule.ID]; ok {
		return err
	}
	if _, ok := m.rules[rule.ID]; !ok {
		return domain.ErrAlertRuleNotFound
	}
	copied := *rule
	m.rules[rule.ID] = &copied
	return nil
}

func (m *mockAlertRuleRepository) Delete(_ context.Context, id string) error {
	m.deleteCalls++
	if err, ok := m.deleteErrByID[id]; ok {
		return err
	}
	if _, ok := m.rules[id]; !ok {
		return domain.ErrAlertRuleNotFound
	}
	delete(m.rules, id)
	return nil
}

type mockAlertRepository struct{}

func (m *mockAlertRepository) Create(_ context.Context, _ *domain.Alert) error { return nil }
func (m *mockAlertRepository) List(_ context.Context) ([]*domain.Alert, error) {
	return []*domain.Alert{}, nil
}

func newAlertEcho(t *testing.T, ruleRepo *mockAlertRuleRepository) *echo.Echo {
	t.Helper()

	e := echo.New()
	createAlertRuleUC := usecase.NewCreateAlertRule(ruleRepo)
	getAlertRuleUC := usecase.NewGetAlertRule(ruleRepo)
	listAlertRulesUC := usecase.NewListAlertRules(ruleRepo)
	updateAlertRuleUC := usecase.NewUpdateAlertRule(ruleRepo)
	deleteAlertRuleUC := usecase.NewDeleteAlertRule(ruleRepo)
	listAlertsUC := usecase.NewListAlerts(&mockAlertRepository{})
	h := obshandler.NewAlertHandler(createAlertRuleUC, getAlertRuleUC, listAlertRulesUC, updateAlertRuleUC, deleteAlertRuleUC, listAlertsUC)

	v1 := e.Group("/api/v1")
	observability := v1.Group("/observability")
	h.RegisterRoutes(observability)

	return e
}

func TestAlertHandler_GetRule_Success(t *testing.T) {
	repo := newMockAlertRuleRepository(&domain.AlertRule{
		ID:                "alr-1",
		Name:              "cpu",
		MetricName:        "cpu_usage",
		Condition:         "cpu_usage >= critical_threshold",
		WarningThreshold:  70,
		CriticalThreshold: 90,
		Threshold:         90,
		Channel:           domain.AlertChannelSlack,
		Enabled:           true,
	})
	e := newAlertEcho(t, repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/observability/alert-rules/alr-1", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 1, repo.getByIDCalls)

	var resp domain.AlertRule
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "alr-1", resp.ID)
	assert.Equal(t, "cpu", resp.Name)
	assert.Equal(t, "cpu_usage", resp.MetricName)
	assert.Equal(t, 70.0, resp.WarningThreshold)
	assert.Equal(t, 90.0, resp.CriticalThreshold)
}

func TestAlertHandler_UpdateRule_Success(t *testing.T) {
	repo := newMockAlertRuleRepository(&domain.AlertRule{
		ID:                "alr-1",
		Name:              "cpu",
		MetricName:        "cpu_usage",
		WarningThreshold:  75,
		CriticalThreshold: 90,
		Threshold:         90,
		Channel:           domain.AlertChannelSlack,
		Enabled:           true,
	})
	e := newAlertEcho(t, repo)

	body := `{"name":"cpu-updated","warning_threshold":80,"critical_threshold":85,"enabled":false}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/observability/alert-rules/alr-1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 1, repo.updateCalls)
	assert.Equal(t, "cpu-updated", repo.rules["alr-1"].Name)
	assert.Equal(t, 80.0, repo.rules["alr-1"].WarningThreshold)
	assert.Equal(t, 85.0, repo.rules["alr-1"].CriticalThreshold)
	assert.Equal(t, 85.0, repo.rules["alr-1"].Threshold)
	assert.False(t, repo.rules["alr-1"].Enabled)
}

func TestAlertHandler_UpdateRule_NotFound(t *testing.T) {
	repo := newMockAlertRuleRepository()
	e := newAlertEcho(t, repo)

	body := `{"name":"cpu-updated"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/observability/alert-rules/missing", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, 0, repo.updateCalls)

	var resp map[string]map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "ALERT_RULE_NOT_FOUND", resp["error"]["code"])
}

func TestAlertHandler_DeleteRule_Success(t *testing.T) {
	repo := newMockAlertRuleRepository(&domain.AlertRule{ID: "alr-1", Name: "cpu", MetricName: "cpu", Channel: domain.AlertChannelSlack, Enabled: true})
	e := newAlertEcho(t, repo)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/observability/alert-rules/alr-1", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, 1, repo.deleteCalls)
	_, exists := repo.rules["alr-1"]
	assert.False(t, exists)
}

func TestAlertHandler_DeleteRule_NotFound(t *testing.T) {
	repo := newMockAlertRuleRepository()
	e := newAlertEcho(t, repo)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/observability/alert-rules/missing", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, 1, repo.deleteCalls)

	var resp map[string]map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "ALERT_RULE_NOT_FOUND", resp["error"]["code"])
}

func TestAlertHandler_ListRules_Success(t *testing.T) {
	repo := newMockAlertRuleRepository(
		&domain.AlertRule{ID: "alr-2", Name: "memory", MetricName: "mem", WarningThreshold: 70, CriticalThreshold: 80, Threshold: 80, Channel: domain.AlertChannelEmail, Enabled: true},
		&domain.AlertRule{ID: "alr-1", Name: "cpu", MetricName: "cpu", WarningThreshold: 80, CriticalThreshold: 90, Threshold: 90, Channel: domain.AlertChannelSlack, Enabled: true},
	)
	e := newAlertEcho(t, repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/observability/alert-rules", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 1, repo.listCalls)

	var resp struct {
		Items []domain.AlertRule `json:"items"`
		Total int                `json:"total"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 2, resp.Total)
	require.Len(t, resp.Items, 2)
	assert.Equal(t, "alr-1", resp.Items[0].ID)
	assert.Equal(t, "alr-2", resp.Items[1].ID)
}

func TestAlertHandler_CreateRule_Success(t *testing.T) {
	repo := newMockAlertRuleRepository()
	e := newAlertEcho(t, repo)

	body := `{"name":"latency","metric_name":"latency_p95","warning_threshold":250,"critical_threshold":300,"channel":"email","enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/observability/alert-rules", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, 1, repo.createCalls)

	var resp domain.AlertRule
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.ID)
	assert.Equal(t, "latency", resp.Name)
	assert.Equal(t, 250.0, resp.WarningThreshold)
	assert.Equal(t, 300.0, resp.CriticalThreshold)
	assert.Equal(t, domain.AlertChannelEmail, resp.Channel)
	assert.True(t, resp.Enabled)
}
