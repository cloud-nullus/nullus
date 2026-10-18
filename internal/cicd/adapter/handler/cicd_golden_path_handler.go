package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// CICDGoldenPathHandler handles HTTP requests for CI/CD Golden Path operations.
type CICDGoldenPathHandler struct {
	goldenPathRepo port.CICDGoldenPathRepository
}

// NewCICDGoldenPathHandler constructs a CICDGoldenPathHandler.
func NewCICDGoldenPathHandler(goldenPathRepo port.CICDGoldenPathRepository) *CICDGoldenPathHandler {
	return &CICDGoldenPathHandler{goldenPathRepo: goldenPathRepo}
}

// RegisterRoutes registers CI/CD Golden Path routes on the given Echo group.
//
// writeGuards 는 생성·수정·삭제에만 붙는다. 골든패스는 조직 전체가 고르는 설치 조합이라
// 읽기는 /cicd 그룹의 세 역할 모두에게 열어 두고, 바꾸는 쪽은 호출하는 쪽이 좁힌다.
// 미들웨어를 여기서 만들지 않는 것은 cicd 가 auth 모듈을 직접 import 하지 않기 위함이다.
func (h *CICDGoldenPathHandler) RegisterRoutes(g *echo.Group, writeGuards ...echo.MiddlewareFunc) {
	g.GET("/golden-paths", h.ListGoldenPaths)
	g.GET("/golden-paths/:id", h.GetGoldenPath)
	g.POST("/golden-paths", h.CreateGoldenPath, writeGuards...)
	g.PUT("/golden-paths/:id", h.UpdateGoldenPath, writeGuards...)
	g.DELETE("/golden-paths/:id", h.DeleteGoldenPath, writeGuards...)
}

type cicdGoldenPathRequest struct {
	ID                   string            `json:"id"`
	Name                 string            `json:"name"`
	Description          string            `json:"description"`
	Tools                []domain.CICDTool `json:"tools"`
	EstimatedInstallTime int               `json:"estimated_install_time"`
	RecommendedUseCase   string            `json:"recommended_use_case"`
	MinResources         string            `json:"min_resources"`
}

// ListGoldenPaths handles GET /api/v1/cicd/golden-paths.
func (h *CICDGoldenPathHandler) ListGoldenPaths(c echo.Context) error {
	goldenPaths, err := h.goldenPathRepo.List(c.Request().Context())
	if err != nil {
		return errorResponse(c, http.StatusInternalServerError, "CICD_GOLDEN_PATH_LIST_FAILED", err.Error())
	}
	return c.JSON(http.StatusOK, goldenPaths)
}

// GetGoldenPath handles GET /api/v1/cicd/golden-paths/:id.
func (h *CICDGoldenPathHandler) GetGoldenPath(c echo.Context) error {
	id := c.Param("id")

	goldenPath, err := h.goldenPathRepo.GetByID(c.Request().Context(), id)
	if err != nil {
		return goldenPathErrorResponse(c, err, "CICD_GOLDEN_PATH_GET_FAILED")
	}

	return c.JSON(http.StatusOK, goldenPath)
}

// CreateGoldenPath handles POST /api/v1/cicd/golden-paths.
func (h *CICDGoldenPathHandler) CreateGoldenPath(c echo.Context) error {
	var req cicdGoldenPathRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "CICD_GOLDEN_PATH_INVALID", err.Error())
	}

	goldenPath := &domain.CICDGoldenPath{
		ID:                   strings.TrimSpace(req.ID),
		Name:                 strings.TrimSpace(req.Name),
		Description:          req.Description,
		Tools:                req.Tools,
		EstimatedInstallTime: req.EstimatedInstallTime,
		RecommendedUseCase:   req.RecommendedUseCase,
		MinResources:         req.MinResources,
	}

	// ID 와 이름이 없으면 저장하지 않는다. 빈 ID 는 키가 빈 행을 만들고,
	// 이름이 없는 카드는 화면에서 고를 수 없다.
	if err := validateGoldenPath(goldenPath); err != nil {
		return errorResponse(c, http.StatusBadRequest, "CICD_GOLDEN_PATH_INVALID", err.Error())
	}

	if err := h.goldenPathRepo.Create(c.Request().Context(), goldenPath); err != nil {
		return goldenPathErrorResponse(c, err, "CICD_GOLDEN_PATH_CREATE_FAILED")
	}

	return c.JSON(http.StatusCreated, goldenPath)
}

// UpdateGoldenPath handles PUT /api/v1/cicd/golden-paths/:id.
func (h *CICDGoldenPathHandler) UpdateGoldenPath(c echo.Context) error {
	var req cicdGoldenPathRequest
	if err := c.Bind(&req); err != nil {
		return errorResponse(c, http.StatusBadRequest, "CICD_GOLDEN_PATH_INVALID", err.Error())
	}

	goldenPath := &domain.CICDGoldenPath{
		ID:                   c.Param("id"),
		Name:                 strings.TrimSpace(req.Name),
		Description:          req.Description,
		Tools:                req.Tools,
		EstimatedInstallTime: req.EstimatedInstallTime,
		RecommendedUseCase:   req.RecommendedUseCase,
		MinResources:         req.MinResources,
	}

	if err := validateGoldenPath(goldenPath); err != nil {
		return errorResponse(c, http.StatusBadRequest, "CICD_GOLDEN_PATH_INVALID", err.Error())
	}

	if err := h.goldenPathRepo.Update(c.Request().Context(), goldenPath); err != nil {
		return goldenPathErrorResponse(c, err, "CICD_GOLDEN_PATH_UPDATE_FAILED")
	}

	return c.JSON(http.StatusOK, goldenPath)
}

// DeleteGoldenPath handles DELETE /api/v1/cicd/golden-paths/:id.
func (h *CICDGoldenPathHandler) DeleteGoldenPath(c echo.Context) error {
	id := c.Param("id")

	if err := h.goldenPathRepo.Delete(c.Request().Context(), id); err != nil {
		return goldenPathErrorResponse(c, err, "CICD_GOLDEN_PATH_DELETE_FAILED")
	}

	return c.NoContent(http.StatusNoContent)
}

// validateGoldenPath 는 저장 전에 꼭 있어야 하는 두 값을 본다.
func validateGoldenPath(goldenPath *domain.CICDGoldenPath) error {
	if goldenPath.ID == "" {
		return fmt.Errorf("%w: id is required", domain.ErrGoldenPathInvalid)
	}
	if goldenPath.Name == "" {
		return fmt.Errorf("%w: name is required", domain.ErrGoldenPathInvalid)
	}
	return nil
}

// goldenPathErrorResponse 는 저장소 에러를 상태 코드로 옮긴다. 없으면 404, 겹치면 409, 그 밖은 500 과 fallbackCode.
func goldenPathErrorResponse(c echo.Context, err error, fallbackCode string) error {
	switch {
	case errors.Is(err, domain.ErrGoldenPathNotFound):
		return errorResponse(c, http.StatusNotFound, "CICD_GOLDEN_PATH_NOT_FOUND", err.Error())
	case errors.Is(err, domain.ErrGoldenPathAlreadyExists):
		return errorResponse(c, http.StatusConflict, "CICD_GOLDEN_PATH_ALREADY_EXISTS", err.Error())
	case errors.Is(err, domain.ErrGoldenPathInvalid):
		return errorResponse(c, http.StatusBadRequest, "CICD_GOLDEN_PATH_INVALID", err.Error())
	default:
		return errorResponse(c, http.StatusInternalServerError, fallbackCode, err.Error())
	}
}
