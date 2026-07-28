package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/cloud-nullus/draft/pkg/crypto"
)

// defaultStorageClassAnnotations 는 기본 StorageClass 를 표시하는 어노테이션이다.
// beta 키는 구버전 클러스터 호환을 위해 함께 확인한다.
var defaultStorageClassAnnotations = []string{
	"storageclass.kubernetes.io/is-default-class",
	"storageclass.beta.kubernetes.io/is-default-class",
}

// clusterStorageClassResponseItem 은 설치 마법사가 StorageClass 를 고르는 데
// 필요한 정보를 담는다.
//
//   - IsDefault: 기본 선택값 결정. 기본 SC 가 하나도 없으면 UI 가 선택을 강제한다
//   - ReclaimPolicy: Retain 이면 스택 삭제 후에도 볼륨이 남는다는 경고를 띄운다
//   - VolumeBindingMode: WaitForFirstConsumer 면 PVC 가 Pending 으로 보이는 것이 정상이다
type clusterStorageClassResponseItem struct {
	Name              string `json:"name"`
	Provisioner       string `json:"provisioner"`
	IsDefault         bool   `json:"is_default"`
	ReclaimPolicy     string `json:"reclaim_policy"`
	VolumeBindingMode string `json:"volume_binding_mode"`
}

var storageClassListerFn = listStorageClassesFromKubeconfig

// ListStorageClasses 는 대상 클러스터의 StorageClass 목록을 반환한다.
func (h *ClusterHandler) ListStorageClasses(c echo.Context) error {
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

	items, err := storageClassListerFn(decrypted)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}
	if items == nil {
		items = []clusterStorageClassResponseItem{}
	}

	return c.JSON(http.StatusOK, map[string]any{"data": items})
}

func listStorageClassesFromKubeconfig(kubeconfig []byte) ([]clusterStorageClassResponseItem, error) {
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, err
	}
	restConfig.Timeout = 10 * time.Second

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	list, err := clientset.StorageV1().StorageClasses().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	items := make([]clusterStorageClassResponseItem, 0, len(list.Items))
	for _, sc := range list.Items {
		items = append(items, clusterStorageClassResponseItem{
			Name:              sc.Name,
			Provisioner:       sc.Provisioner,
			IsDefault:         isDefaultStorageClass(sc),
			ReclaimPolicy:     reclaimPolicyOf(sc),
			VolumeBindingMode: volumeBindingModeOf(sc),
		})
	}
	return items, nil
}

func isDefaultStorageClass(sc storagev1.StorageClass) bool {
	for _, key := range defaultStorageClassAnnotations {
		if sc.Annotations[key] == "true" {
			return true
		}
	}
	return false
}

func reclaimPolicyOf(sc storagev1.StorageClass) string {
	if sc.ReclaimPolicy == nil {
		// 명시되지 않으면 Kubernetes 기본값은 Delete 다.
		return "Delete"
	}
	return string(*sc.ReclaimPolicy)
}

func volumeBindingModeOf(sc storagev1.StorageClass) string {
	if sc.VolumeBindingMode == nil {
		return "Immediate"
	}
	return string(*sc.VolumeBindingMode)
}
