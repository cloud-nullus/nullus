package handler

import (
	"encoding/base64"
	"net/http"
	"os"

	"github.com/cloud-nullus/draft/internal/admin/adapter/kube"
	"github.com/cloud-nullus/draft/internal/admin/domain"
	"github.com/cloud-nullus/draft/internal/admin/usecase"
	"github.com/cloud-nullus/draft/internal/shared/audit"
	"github.com/cloud-nullus/draft/pkg/crypto"
	"github.com/labstack/echo/v4"
)

// ClusterHandler handles HTTP requests for clusters.
type ClusterHandler struct {
	clusterUC     *usecase.ClusterUseCase
	audit         *audit.AuditLogger
	encryptionKey []byte
}

// NewClusterHandler creates a new ClusterHandler.
func NewClusterHandler(clusterUC *usecase.ClusterUseCase, auditLogger ...*audit.AuditLogger) *ClusterHandler {
	var logger *audit.AuditLogger
	if len(auditLogger) > 0 {
		logger = auditLogger[0]
	}
	return &ClusterHandler{
		clusterUC:     clusterUC,
		audit:         logger,
		encryptionKey: []byte(os.Getenv("ENCRYPTION_KEY")),
	}
}

type registerClusterRequest struct {
	Name       string             `json:"name"`
	Type       domain.ClusterType `json:"type"`
	Endpoint   string             `json:"endpoint"`
	OrgID      string             `json:"org_id"`
	Kubeconfig string             `json:"kubeconfig"`
}

type updateClusterRequest struct {
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"`
}

type clusterResponse struct {
	ID               string                  `json:"id"`
	Name             string                  `json:"name"`
	Type             domain.ClusterType      `json:"type"`
	Endpoint         string                  `json:"endpoint"`
	ConnectionStatus domain.ConnectionStatus `json:"connection_status"`
	OrgID            string                  `json:"org_id"`
	Kubeconfig       string                  `json:"kubeconfig,omitempty"`
	CreatedAt        any                     `json:"created_at"`
	UpdatedAt        any                     `json:"updated_at"`
}

func toClusterResponse(cluster *domain.Cluster, kubeconfig string) clusterResponse {
	return clusterResponse{
		ID:               cluster.ID,
		Name:             cluster.Name,
		Type:             cluster.Type,
		Endpoint:         cluster.Endpoint,
		ConnectionStatus: cluster.ConnectionStatus,
		OrgID:            cluster.OrgID,
		Kubeconfig:       kubeconfig,
		CreatedAt:        cluster.CreatedAt,
		UpdatedAt:        cluster.UpdatedAt,
	}
}

// RegisterCluster handles POST /api/v1/clusters.
func (h *ClusterHandler) RegisterCluster(c echo.Context) error {
	var req registerClusterRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	orgID := req.OrgID
	if orgID == "" {
		firstOrg, err := h.clusterUC.GetFirstOrgID(c.Request().Context())
		if err == nil && firstOrg != "" {
			orgID = firstOrg
		}
	}

	cluster, err := h.clusterUC.RegisterCluster(c.Request().Context(), usecase.RegisterClusterInput{
		Name:     req.Name,
		Type:     req.Type,
		Endpoint: req.Endpoint,
		OrgID:    orgID,
	})
	if err != nil {
		return err
	}

	if req.Kubeconfig != "" && len(h.encryptionKey) == 32 {
		// Try base64 decode first; if it fails, treat as raw YAML
		kubeconfigBytes := []byte(req.Kubeconfig)
		if decoded, err := base64.StdEncoding.DecodeString(req.Kubeconfig); err == nil {
			kubeconfigBytes = decoded
		}
		encrypted, err := crypto.Encrypt(h.encryptionKey, kubeconfigBytes)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to encrypt kubeconfig")
		}
		if err := h.clusterUC.SaveKubeconfig(c.Request().Context(), cluster.ID, []byte(encrypted)); err != nil {
			return err
		}
	}
	if h.audit != nil {
		_ = h.audit.Log(c.Request().Context(), audit.AuditEntry{
			UserID:       c.Request().Header.Get("X-User-ID"),
			Action:       "create",
			ResourceType: "cluster",
			ResourceID:   cluster.ID,
			Details: map[string]any{
				"name":     req.Name,
				"type":     req.Type,
				"endpoint": req.Endpoint,
				"org_id":   req.OrgID,
			},
			IPAddress: c.RealIP(),
		})
	}

	return c.JSON(http.StatusCreated, toClusterResponse(cluster, req.Kubeconfig))
}

// ListClusters handles GET /api/v1/clusters.
func (h *ClusterHandler) ListClusters(c echo.Context) error {
	orgID := c.QueryParam("org_id")

	clusters, err := h.clusterUC.ListClusters(c.Request().Context(), orgID)
	if err != nil {
		return err
	}

	items := make([]clusterResponse, 0, len(clusters))
	for _, cluster := range clusters {
		items = append(items, toClusterResponse(cluster, ""))
	}

	return c.JSON(http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

// GetCluster handles GET /api/v1/clusters/:id.
func (h *ClusterHandler) GetCluster(c echo.Context) error {
	id := c.Param("id")

	cluster, err := h.clusterUC.GetCluster(c.Request().Context(), id)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, toClusterResponse(cluster, ""))
}

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
	if h.audit != nil {
		_ = h.audit.Log(c.Request().Context(), audit.AuditEntry{
			UserID:       c.Request().Header.Get("X-User-ID"),
			Action:       "update",
			ResourceType: "cluster",
			ResourceID:   cluster.ID,
			Details: map[string]any{
				"name":     req.Name,
				"endpoint": req.Endpoint,
			},
			IPAddress: c.RealIP(),
		})
	}

	return c.JSON(http.StatusOK, toClusterResponse(cluster, ""))
}

// DeleteCluster handles DELETE /api/v1/clusters/:id.
func (h *ClusterHandler) DeleteCluster(c echo.Context) error {
	id := c.Param("id")

	if err := h.clusterUC.DeleteCluster(c.Request().Context(), id); err != nil {
		return err
	}
	if h.audit != nil {
		_ = h.audit.Log(c.Request().Context(), audit.AuditEntry{
			UserID:       c.Request().Header.Get("X-User-ID"),
			Action:       "delete",
			ResourceType: "cluster",
			ResourceID:   id,
			Details:      map[string]any{},
			IPAddress:    c.RealIP(),
		})
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *ClusterHandler) VerifyCluster(c echo.Context) error {
	id := c.Param("id")

	encryptedConfig, err := h.clusterUC.GetKubeconfig(c.Request().Context(), id)
	if err != nil {
		return err
	}
	if len(encryptedConfig) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "kubeconfig is not registered for this cluster")
	}
	if len(h.encryptionKey) != 32 {
		return echo.NewHTTPError(http.StatusInternalServerError, "ENCRYPTION_KEY must be 32 bytes")
	}

	decrypted, err := crypto.Decrypt(h.encryptionKey, string(encryptedConfig))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to decrypt kubeconfig")
	}

	result, err := kube.VerifyCluster(decrypted)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}

	if _, err := h.clusterUC.VerifyCluster(c.Request().Context(), id); err != nil {
		return err
	}
	if h.audit != nil {
		_ = h.audit.Log(c.Request().Context(), audit.AuditEntry{
			UserID:       c.Request().Header.Get("X-User-ID"),
			Action:       "verify",
			ResourceType: "cluster",
			ResourceID:   id,
			Details: map[string]any{
				"status":  result.Status,
				"version": result.Version,
			},
			IPAddress: c.RealIP(),
		})
	}

	return c.JSON(http.StatusOK, result)
}

// RegisterRoutes registers cluster routes on the given group.
func (h *ClusterHandler) RegisterRoutes(g *echo.Group) {
	g.POST("/clusters", h.RegisterCluster)
	g.GET("/clusters", h.ListClusters)
	g.GET("/clusters/:id", h.GetCluster)
	g.PATCH("/clusters/:id", h.UpdateCluster)
	g.DELETE("/clusters/:id", h.DeleteCluster)
	g.POST("/clusters/:id/verify", h.VerifyCluster)
}
