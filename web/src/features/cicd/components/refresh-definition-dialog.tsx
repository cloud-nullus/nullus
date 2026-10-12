import { useTranslation } from "react-i18next";

import { Modal } from "../../../components/ui/modal";
import { Button } from "../../../components/ui/button";

interface RefreshDefinitionDialogProps {
  open: boolean;
  pipelineName: string;
  busy?: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}

/**
 * CI 정의 갱신 확인 대화상자.
 *
 * SAST 리포트처럼 단계가 생기기 전에 만든 파이프라인은 정의 파일을 플랫폼 기본으로 다시
 * 써야 실행 기록에 결과가 붙는다 — 전에는 파이프라인을 다시 만드는 것 말고 방법이 없었다.
 * 사용자가 손본 정의 파일은 덮어쓰므로, 무엇을 덮어쓰고 무엇은 건드리지 않는지를 말하고
 * 확인을 받는다.
 */
export function RefreshDefinitionDialog({
  open,
  pipelineName,
  busy = false,
  onCancel,
  onConfirm,
}: RefreshDefinitionDialogProps) {
  const { t } = useTranslation();
  if (!open) return null;

  return (
    <Modal
      open={open}
      onClose={onCancel}
      title={t("cicd.refreshDefinitionDialog.title", "CI 정의 갱신")}
      footer={
        <div className="flex justify-end gap-2">
          <Button variant="outline" onClick={onCancel} disabled={busy}>
            {t("common.cancel", "취소")}
          </Button>
          <Button variant="primary" onClick={onConfirm} disabled={busy}>
            {t("cicd.refreshDefinitionDialog.confirm", "정의 갱신")}
          </Button>
        </div>
      }
    >
      <div className="space-y-4 text-sm">
        <p>
          {t("cicd.refreshDefinitionDialog.intro", {
            defaultValue: '파이프라인 "{{name}}" 의 CI 정의 파일을 플랫폼 기본으로 다시 씁니다.',
            name: pipelineName,
          })}
        </p>
        <p>
          {t(
            "cicd.refreshDefinitionDialog.overwrites",
            "덮어쓰는 것: 이 스택의 파이프라인 정의 파일(.gitlab-ci.yml · .github/workflows/nullus-ci.yml · Jenkinsfile 중 하나)과 SONAR_TOKEN 같은 CI 변수. 직접 고친 내용은 사라집니다.",
          )}
        </p>
        <p>
          {t(
            "cicd.refreshDefinitionDialog.keeps",
            "건드리지 않는 것: 앱 코드 · Dockerfile · deploy/ 매니페스트 · Argo CD Application.",
          )}
        </p>
        <p className="text-xs text-muted-foreground">
          {t(
            "cicd.refreshDefinitionDialog.why",
            "SAST 리포트처럼 단계가 생기기 전에 만든 파이프라인은 이렇게 해야 실행 기록에 결과가 붙습니다.",
          )}
        </p>
      </div>
    </Modal>
  );
}
