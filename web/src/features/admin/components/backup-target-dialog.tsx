import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Modal } from "../../../components/ui/modal";
import { Button } from "../../../components/ui/button";
import { Checkbox } from "../../../components/ui/checkbox";
import { TextInput } from "../../../components/ui/text-input";
import { StatusIcon } from "../../../components/ui/status-icon";
import { requiresQuiesce, type BackupComponent } from "../api/backup-api";

/**
 * 고를 수 있는 대상과, 그것이 무엇인지.
 *
 * 이름만으로는 무엇이 빠지는지 알 수 없다 — "금고" 를 빼면 복구 후 도구들이
 * 자격증명을 못 찾는다는 것을 화면에서 말해 주지 않으면, 사용자는 용량을
 * 줄이려다 복구를 못 하게 된다.
 */
const TARGETS: { id: BackupComponent; labelKey: string; descKey: string }[] = [
  {
    id: "platform_db",
    labelKey: "backupPage.target.platformDb",
    descKey: "backupPage.target.platformDbDesc",
  },
  {
    id: "keycloak_db",
    labelKey: "backupPage.target.keycloakDb",
    descKey: "backupPage.target.keycloakDbDesc",
  },
  {
    id: "openbao_kv",
    labelKey: "backupPage.target.openbaoKv",
    descKey: "backupPage.target.openbaoKvDesc",
  },
  {
    id: "ns_resources",
    labelKey: "backupPage.target.nsResources",
    descKey: "backupPage.target.nsResourcesDesc",
  },
  {
    id: "volume",
    labelKey: "backupPage.target.volume",
    descKey: "backupPage.target.volumeDesc",
  },
];

const ALL: BackupComponent[] = TARGETS.map((t) => t.id);

export interface BackupTargetDialogProps {
  open: boolean;
  onClose: () => void;
  /** 확인 문구로 입력해야 하는 네임스페이스. 정지 창이 생길 때만 쓰인다. */
  namespace: string;
  submitting?: boolean;
  onSubmit: (input: { scope: BackupComponent[]; confirm?: string }) => void;
}

export function BackupTargetDialog({
  open,
  onClose,
  namespace,
  submitting = false,
  onSubmit,
}: BackupTargetDialogProps) {
  const { t } = useTranslation();
  const [scope, setScope] = useState<BackupComponent[]>(ALL);
  const [confirm, setConfirm] = useState("");

  const stops = useMemo(() => requiresQuiesce(scope), [scope]);
  const nothingSelected = scope.length === 0;
  // 정지 창이 생길 때만 확인을 요구한다. 멈추지도 않는데 겁을 주면 확인
  // 문구는 의미 없는 절차가 되고, 정작 진짜 멈추는 백업에서도 습관적으로
  // 넘기게 된다.
  const confirmOk = !stops || confirm.trim() === namespace;
  const canSubmit = !nothingSelected && confirmOk && !submitting;

  const toggle = (id: BackupComponent) =>
    setScope((prev) =>
      prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id],
    );

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={t("backupPage.dialog.title", "Select backup targets")}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            {t("common.cancel", "Cancel")}
          </Button>
          <Button
            onClick={() =>
              onSubmit({ scope, confirm: stops ? confirm.trim() : undefined })
            }
            disabled={!canSubmit}
          >
            {t("backupPage.dialog.start", "Start backup")}
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-3">
        <p className="text-sm text-[var(--color-text-secondary)]">
          {t(
            "backupPage.dialog.description",
            "Choose what to include in this backup.",
          )}
        </p>

        <ul className="flex flex-col gap-2">
          {TARGETS.map((target) => (
            <li key={target.id} className="flex gap-2">
              <Checkbox
                id={`backup-target-${target.id}`}
                align="start"
                checked={scope.includes(target.id)}
                onChange={() => toggle(target.id)}
              />
              <label
                htmlFor={`backup-target-${target.id}`}
                className="cursor-pointer"
              >
                <span className="block text-sm font-medium">
                  {t(target.labelKey, target.id)}
                </span>
                <span className="block text-xs text-[var(--color-text-secondary)]">
                  {t(target.descKey, "")}
                </span>
              </label>
            </li>
          ))}
        </ul>

        {nothingSelected && (
          <p role="alert" className="text-xs text-[var(--color-error)]">
            {t(
              "backupPage.dialog.nothingSelected",
              "Select at least one target.",
            )}
          </p>
        )}

        {stops && (
          <div
            role="alert"
            className="flex gap-2 rounded-[var(--card-radius)] border border-[var(--color-warning)] p-3"
          >
            <StatusIcon tone="warning" className="shrink-0" />
            <div className="flex flex-col gap-2">
              <span className="text-sm">
                {t(
                  "backupPage.dialog.quiesceWarning",
                  "Including volumes stops the workloads while the backup runs.",
                )}
              </span>
              <label className="text-xs" htmlFor="backup-confirm">
                {t(
                  "backupPage.dialog.confirmLabel",
                  "Type the namespace to confirm",
                )}
                : <code>{namespace}</code>
              </label>
              <TextInput
                id="backup-confirm"
                value={confirm}
                onChange={(e) => setConfirm(e.target.value)}
                placeholder={namespace}
              />
            </div>
          </div>
        )}
      </div>
    </Modal>
  );
}
