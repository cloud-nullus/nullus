import { useEffect, useState } from "react";
import { AlertTriangle } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Modal } from "../../../components/ui/modal";
import { Button } from "../../../components/ui/button";
import { Input } from "../../../components/ui/input";

export interface DeletePipelineSelection {
  deleteClusterResources: boolean;
  deleteRepository: boolean;
  deleteImages: boolean;
}

interface DeletePipelineDialogProps {
  open: boolean;
  pipelineName: string;
  /** 저장소 경로(owner/repo). 없으면 저장소 삭제 선택지를 감춘다. */
  repositoryPath?: string;
  /** 이미지 저장소 경로. 없으면 이미지 삭제 선택지를 감춘다. */
  imageRepository?: string;
  busy?: boolean;
  onCancel: () => void;
  onConfirm: (selection: DeletePipelineSelection) => void;
}

/**
 * 파이프라인 삭제 확인 대화상자.
 *
 * 부수 리소스는 기본으로 지우지 않는다. 파이프라인 레코드만 지우는 것이 종전
 * 동작이고, 되돌릴 수 없는 일(저장소·이미지)과 서비스가 멈추는 일(클러스터
 * 리소스)은 사용자가 하나씩 골라야 한다.
 *
 * 저장소를 고르면 이름을 그대로 입력해야 확인 버튼이 열린다 — 목록에서 잘못된
 * 행의 삭제 버튼을 눌렀을 때 마지막으로 걸러지는 지점이다.
 */
export function DeletePipelineDialog({
  open,
  pipelineName,
  repositoryPath,
  imageRepository,
  busy = false,
  onCancel,
  onConfirm,
}: DeletePipelineDialogProps) {
  const { t } = useTranslation();
  const [deleteClusterResources, setDeleteClusterResources] = useState(false);
  const [deleteRepository, setDeleteRepository] = useState(false);
  const [deleteImages, setDeleteImages] = useState(false);
  const [typedRepository, setTypedRepository] = useState("");

  // 대화상자를 다시 열 때 이전 선택이 남아 있으면, 확인만 누르던 사용자가
  // 의도치 않게 저장소를 지운다.
  useEffect(() => {
    if (!open) return;
    setDeleteClusterResources(false);
    setDeleteRepository(false);
    setDeleteImages(false);
    setTypedRepository("");
  }, [open]);

  const repositoryConfirmed =
    !deleteRepository ||
    (!!repositoryPath && typedRepository.trim() === repositoryPath);
  const canConfirm = !busy && repositoryConfirmed;

  return (
    <Modal
      open={open}
      onClose={onCancel}
      title={t("cicd.deleteDialog.title", "파이프라인 삭제")}
      footer={
        <div className="flex justify-end gap-2">
          <Button variant="outline" onClick={onCancel} disabled={busy}>
            {t("common.cancel", "취소")}
          </Button>
          <Button
            variant="danger"
            disabled={!canConfirm}
            onClick={() =>
              onConfirm({
                deleteClusterResources,
                deleteRepository,
                deleteImages,
              })
            }
          >
            {t("cicd.deleteDialog.confirm", "삭제")}
          </Button>
        </div>
      }
    >
      <div className="space-y-4 text-sm">
        <p>
          {t("cicd.deleteDialog.intro", {
            defaultValue:
              '파이프라인 "{{name}}" 과 배포 이력을 지웁니다.',
            name: pipelineName,
          })}
        </p>

        <div className="space-y-3 rounded-md border border-border p-3">
          <p className="font-medium">
            {t("cicd.deleteDialog.alsoDelete", "함께 지울 항목")}
          </p>

          <label className="flex items-start gap-2">
            <input
              type="checkbox"
              className="mt-1"
              checked={deleteClusterResources}
              onChange={(e) => setDeleteClusterResources(e.target.checked)}
            />
            <span>
              <span className="block">
                {t("cicd.deleteDialog.clusterResources", "클러스터 리소스")}
              </span>
              <span className="block text-xs text-muted-foreground">
                {t("cicd.deleteDialog.clusterResourcesHint", {
                  defaultValue:
                    "Argo CD Application 과 배포된 워크로드를 지웁니다. 고르지 않으면 앱은 계속 실행됩니다.",
                })}
              </span>
            </span>
          </label>

          {imageRepository ? (
            <label className="flex items-start gap-2">
              <input
                type="checkbox"
                className="mt-1"
                checked={deleteImages}
                onChange={(e) => setDeleteImages(e.target.checked)}
              />
              <span>
                <span className="block">
                  {t("cicd.deleteDialog.images", "컨테이너 이미지")}
                </span>
                <span className="block break-all text-xs text-muted-foreground">
                  {imageRepository}
                </span>
              </span>
            </label>
          ) : null}

          {repositoryPath ? (
            <label className="flex items-start gap-2">
              <input
                type="checkbox"
                className="mt-1"
                checked={deleteRepository}
                onChange={(e) => setDeleteRepository(e.target.checked)}
              />
              <span>
                <span className="block">
                  {t("cicd.deleteDialog.repository", "소스 저장소")}
                </span>
                <span className="block break-all text-xs text-muted-foreground">
                  {repositoryPath}
                </span>
              </span>
            </label>
          ) : null}
        </div>

        {deleteRepository && repositoryPath ? (
          <div className="space-y-2 rounded-md border border-destructive/40 bg-destructive/5 p-3">
            <p className="flex items-center gap-2 font-medium text-destructive">
              <AlertTriangle className="h-4 w-4" aria-hidden="true" />
              {t("cicd.deleteDialog.irreversible", "되돌릴 수 없습니다")}
            </p>
            <p className="text-xs">
              {t("cicd.deleteDialog.typeToConfirm", {
                defaultValue:
                  "확인을 위해 저장소 이름을 입력하세요: {{path}}",
                path: repositoryPath,
              })}
            </p>
            <Input
              value={typedRepository}
              onChange={(e) => setTypedRepository(e.target.value)}
              placeholder={repositoryPath}
              aria-label={t(
                "cicd.deleteDialog.typeToConfirmLabel",
                "저장소 이름 확인",
              )}
            />
          </div>
        ) : null}
      </div>
    </Modal>
  );
}
