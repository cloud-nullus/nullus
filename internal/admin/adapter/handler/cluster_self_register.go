package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	adminkube "github.com/cloud-nullus/draft/internal/admin/adapter/kube"
	"github.com/cloud-nullus/draft/internal/admin/domain"
	"github.com/cloud-nullus/draft/internal/admin/usecase"
	"github.com/cloud-nullus/draft/pkg/crypto"
)

// selfRegisterRequest 는 자기 클러스터 등록 요청이다.
// kubeconfig 는 받지 않는다 — 파드가 자기 자격으로 만들기 때문이다.
type selfRegisterRequest struct {
	Name  string `json:"name"`
	OrgID string `json:"org_id"`
}

// inClusterKubeconfigFn 은 테스트에서 교체할 수 있도록 변수로 둔다.
var inClusterKubeconfigFn = adminkube.InClusterKubeconfig

// SelfRegisterCluster 는 Nullus 가 떠 있는 클러스터를 대상 클러스터로 등록한다.
//
// 에어갭 무인 설치와 단일 클러스터 배포에서는 운영자가 업로드할 kubeconfig 의
// 대상이 자기 자신이라 업로드 단계 자체가 불필요하다. 파드에 마운트된
// ServiceAccount 토큰으로 kubeconfig 를 구성하므로 새 자격증명이 생기지 않는다.
//
// 멱등하다 — 같은 이름의 self 클러스터가 이미 있으면 그것을 돌려준다.
func (h *ClusterHandler) SelfRegisterCluster(c echo.Context) error {
	var req selfRegisterRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "nullus-self"
	}
	orgID := strings.TrimSpace(req.OrgID)
	if orgID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "org_id is required")
	}

	ctx := c.Request().Context()

	// 이미 등록되어 있으면 그대로 돌려준다. 아키텍처를 모르는 채 등록된 클러스터(탐색이
	// 실패했거나 이 경로가 탐색을 하지 않던 때 등록된 것)는 여기서 다시 탐색한다.
	if existing, err := h.findSelfCluster(c, orgID, name); err == nil && existing != nil {
		if len(existing.NodeArchitectures) == 0 {
			existing = h.discoverSelfCluster(c, existing)
		}
		return c.JSON(http.StatusOK, toClusterResponse(existing, ""))
	}

	kubeconfig, err := inClusterKubeconfigFn()
	if err != nil {
		return echo.NewHTTPError(http.StatusPreconditionFailed,
			"자기 클러스터 등록은 클러스터 안에서 실행 중일 때만 가능합니다: "+err.Error())
	}

	cluster, err := h.clusterUC.RegisterCluster(ctx, usecase.RegisterClusterInput{
		Name:          name,
		Type:          domain.ClusterTypeSelf,
		Types:         []domain.ClusterType{domain.ClusterTypeSelf, domain.ClusterTypeTarget},
		CloudProvider: domain.CloudProviderOnPremise,
		Endpoint:      "https://kubernetes.default.svc",
		OrgID:         orgID,
	})
	if err != nil {
		return err
	}

	if len(h.encryptionKey) != 32 {
		return echo.NewHTTPError(http.StatusInternalServerError, "ENCRYPTION_KEY must be 32 bytes")
	}
	encrypted, err := crypto.Encrypt(h.encryptionKey, kubeconfig)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to encrypt kubeconfig")
	}
	if err := h.clusterUC.SaveKubeconfig(ctx, cluster.ID, []byte(encrypted)); err != nil {
		return err
	}
	cluster = h.discoverSelfCluster(c, cluster)

	return c.JSON(http.StatusCreated, toClusterResponse(cluster, ""))
}

// discoverSelfCluster 는 노드 아키텍처를 탐색해 저장한다.
//
// 업로드 등록은 탐색을 하는데 셀프 등록만 하지 않았다. node_architectures 가 비면
// Pre-Deploy Gate 는 CLUSTER_ARCH_UNKNOWN warn 만 내고, 무인 설치(29)는 warn 에
// 동의하고 진행한다 — DGX Spark(arm64)에서 amd64 전용 Harbor 가 그렇게 게이트를
// 지나갔다(#270).
//
// 실패해도 등록은 유지한다(업로드 경로와 같은 정책). 설치 직전 사전검사가 노드를
// 직접 읽으므로 여기서 막을 이유는 없다.
func (h *ClusterHandler) discoverSelfCluster(c echo.Context, cluster *domain.Cluster) *domain.Cluster {
	refreshed, err := h.clusterUC.RefreshDiscovery(c.Request().Context(), cluster.ID)
	if err != nil {
		slog.Warn("self cluster discovery failed", "cluster_id", cluster.ID, "error", err)
	}
	// 실패해도 RefreshDiscovery 는 connection_failed 로 저장한 클러스터를 돌려준다.
	if refreshed == nil {
		return cluster
	}
	return refreshed
}

// findSelfCluster 는 같은 조직에 이미 등록된 self 클러스터를 찾는다.
func (h *ClusterHandler) findSelfCluster(c echo.Context, orgID, name string) (*domain.Cluster, error) {
	clusters, err := h.clusterUC.ListClusters(c.Request().Context(), orgID)
	if err != nil {
		return nil, err
	}
	for _, cluster := range clusters {
		if cluster == nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(cluster.Name), name) &&
			cluster.Type == domain.ClusterTypeSelf {
			return cluster, nil
		}
	}
	return nil, nil
}
