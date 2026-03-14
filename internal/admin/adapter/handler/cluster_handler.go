package handler

import (
	"net/http"

	"github.com/cloud-nullus/draft/internal/admin/domain"
	"github.com/cloud-nullus/draft/internal/admin/usecase"
	"github.com/labstack/echo/v4"
)

// ClusterHandler handles HTTP requests for clusters.
type ClusterHandler struct {
	clusterUC *usecase.ClusterUseCase
}

// NewClusterHandler creates a new ClusterHandler.
func NewClusterHandler(clusterUC *usecase.ClusterUseCase) *ClusterHandler {
	return &ClusterHandler{clusterUC: clusterUC}
}

type registerClusterRequest struct {
	Name     string             `json:"name"`
	Type     domain.ClusterType `json:"type"`
	Endpoint string             `json:"endpoint"`
	OrgID    string             `json:"org_id"`
}

type updateClusterRequest struct {
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"`
}

// RegisterCluster handles POST /api/v1/clusters.
func (h *ClusterHandler) RegisterCluster(c echo.Context) error {
	var req registerClusterRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	cluster, err := h.clusterUC.RegisterCluster(c.Request().Context(), usecase.RegisterClusterInput{
		Name:     req.Name,
		Type:     req.Type,
		Endpoint: req.Endpoint,
		OrgID:    req.OrgID,
	})
	if err != nil {
		return err
	}

	return c.JSON(http.StatusCreated, map[string]any{"data": cluster})
}

// ListClusters handles GET /api/v1/clusters.
func (h *ClusterHandler) ListClusters(c echo.Context) error {
	orgID := c.QueryParam("org_id")

	clusters, err := h.clusterUC.ListClusters(c.Request().Context(), orgID)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]any{"data": clusters})
}

// GetCluster handles GET /api/v1/clusters/:id.
func (h *ClusterHandler) GetCluster(c echo.Context) error {
	id := c.Param("id")

	cluster, err := h.clusterUC.GetCluster(c.Request().Context(), id)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]any{"data": cluster})
}

// UpdateCluster handles PUT /api/v1/clusters/:id.
func (h *ClusterHandler) UpdateCluster(c echo.Context) error {
	id := c.Param("id")

	var req updateClusterRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	cluster, err := h.clusterUC.UpdateCluster(c.Request().Context(), id, usecase.UpdateClusterInput{
		Name:     req.Name,
		Endpoint: req.Endpoint,
	})
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]any{"data": cluster})
}

// DeleteCluster handles DELETE /api/v1/clusters/:id.
func (h *ClusterHandler) DeleteCluster(c echo.Context) error {
	id := c.Param("id")

	if err := h.clusterUC.DeleteCluster(c.Request().Context(), id); err != nil {
		return err
	}

	return c.NoContent(http.StatusNoContent)
}

// VerifyCluster handles POST /api/v1/clusters/:id/verify.
func (h *ClusterHandler) VerifyCluster(c echo.Context) error {
	id := c.Param("id")

	cluster, err := h.clusterUC.VerifyCluster(c.Request().Context(), id)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]any{"data": cluster})
}

// RegisterRoutes registers cluster routes on the given group.
func (h *ClusterHandler) RegisterRoutes(g *echo.Group) {
	g.POST("/clusters", h.RegisterCluster)
	g.GET("/clusters", h.ListClusters)
	g.GET("/clusters/:id", h.GetCluster)
	g.PUT("/clusters/:id", h.UpdateCluster)
	g.DELETE("/clusters/:id", h.DeleteCluster)
	g.POST("/clusters/:id/verify", h.VerifyCluster)
}
