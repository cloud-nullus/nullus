import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Modal } from "../../../components/ui/modal";
import { Button } from "../../../components/ui/button";
import { Checkbox } from "../../../components/ui/checkbox";
import { TextInput } from "../../../components/ui/text-input";
import { StatusIcon } from "../../../components/ui/status-icon";
import {
  SET_CONFIRM_PHRASE,
  requiresQuiesce,
  type BackupComponent,
} from "../api/backup-api";

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
    // 클러스터의 마지막 스택을 지우면 회수되는, Nullus 가 설치한 공용 자원. 네임스페이스
    // 리소스가 기대는 것이라 복구는 이것을 먼저, 없을 때만 되살린다.
    id: "shared_resources",
    labelKey: "backupPage.target.sharedResources",
    descKey: "backupPage.target.sharedResourcesDesc",
  },
  {
    id: "volume",
    labelKey: "backupPage.target.volume",
    descKey: "backupPage.target.volumeDesc",
  },
];

const ALL: BackupComponent[] = TARGETS.map((t) => t.id);

/**
 * 스택이 있어야 뜰 수 있는 대상. 금고는 스택마다 배포되고, 리소스·볼륨은 스택
 * 클러스터에서 뜬다 — 서버는 스택 ID 로 그 클러스터를 찾는다.
 */
const STACK_TARGETS: ReadonlySet<BackupComponent> = new Set([
  "openbao_kv",
  "ns_resources",
  "shared_resources",
  "volume",
]);
const PLATFORM: BackupComponent[] = ALL.filter((id) => !STACK_TARGETS.has(id));

/**
 * "전체" 선택의 라디오 값. 스택 ID 와 겹치지 않는 값이면 된다 — 스택 ID 는 stk_ 로 시작한다.
 *
 * 전체는 플랫폼 + 조직의 모든 설치 완료 스택을 한 세트로 뜬다. 어느 스택인지는 서버가 정하므로
 * 대상 체크박스는 고를 수 없고(언제나 전부), 확인 문구는 네임스페이스가 아니라 고정 문구다.
 */
export const ALL_STACKS = "*";

/** 백업할 수 있는 스택. 네임스페이스는 볼륨을 뜰 때 확인 문구가 된다. */
export interface BackupStackOption {
  id: string;
  name: string;
  namespace: string;
}

export interface BackupTargetDialogProps {
  open: boolean;
  onClose: () => void;
  /** 고를 수 있는 스택. 없으면 플랫폼만 뜬다. */
  stacks: BackupStackOption[];
  submitting?: boolean;
  /**
   * all 이면 플랫폼 + 모든 스택을 한 세트로 뜨는 백업이다(stackId·namespace·scope 는 쓰지 않는다).
   * 아니면서 stackId·namespace 가 비어 있으면 플랫폼만 뜨는 백업이다.
   */
  onSubmit: (input: {
    all?: boolean;
    stackId?: string;
    namespace?: string;
    scope: BackupComponent[];
    confirm?: string;
  }) => void;
}

export function BackupTargetDialog({
  open,
  onClose,
  stacks,
  submitting = false,
  onSubmit,
}: BackupTargetDialogProps) {
  const { t } = useTranslation();
  // 빈 문자열은 "플랫폼만" 이다. 스택이 있으면 첫 스택을 고른다 — 볼륨까지 담는 완전한
  // 백업이 기본이어야 한다.
  const [stackId, setStackId] = useState(stacks[0]?.id ?? "");
  const [scope, setScope] = useState<BackupComponent[]>(
    stacks.length > 0 ? ALL : PLATFORM,
  );
  const [confirm, setConfirm] = useState("");

  const all = stackId === ALL_STACKS;
  const stack = all ? undefined : stacks.find((s) => s.id === stackId);
  const namespace = stack?.namespace ?? "";

  const chooseStack = (id: string) => {
    setStackId(id);
    // 대상이 바뀌면 고를 수 있는 것이 바뀐다. 앞 대상의 확인 문구는 이 대상과 맞지 않는다.
    setScope(id ? ALL : PLATFORM);
    setConfirm("");
  };

  // 전체는 스택마다 볼륨까지 뜨므로 언제나 멈춘다.
  const stops = useMemo(() => all || requiresQuiesce(scope), [all, scope]);
  const nothingSelected = scope.length === 0;
  // 정지 창이 생길 때만 확인을 요구한다. 멈추지도 않는데 겁을 주면 확인
  // 문구는 의미 없는 절차가 되고, 정작 진짜 멈추는 백업에서도 습관적으로
  // 넘기게 된다.
  const confirmPhrase = all ? SET_CONFIRM_PHRASE : namespace;
  const confirmOk = !stops || confirm.trim() === confirmPhrase;
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
              onSubmit({
                all: all || undefined,
                stackId: stack?.id,
                namespace: stack?.namespace,
                scope,
                confirm: stops ? confirm.trim() : undefined,
              })
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

        <fieldset className="m-0 flex flex-col gap-1 border-0 p-0">
          <legend className="mb-1 text-sm font-medium">
            {t("backupPage.dialog.stackLabel", "Backup target")}
          </legend>
          {/* 스택이 없으면 전체는 플랫폼만과 같다 — 보여 줄 이유가 없다. */}
          {stacks.length > 0 && (
            <label className="flex cursor-pointer items-center gap-2 text-sm">
              <input
                type="radio"
                className="h-4 w-4 accent-[var(--color-primary)]"
                name="backup-stack"
                value={ALL_STACKS}
                checked={all}
                onChange={() => chooseStack(ALL_STACKS)}
              />
              <span>
                {t("backupPage.dialog.allStacks", "Everything — platform + all stacks")}
              </span>
              <code className="text-xs text-[var(--color-text-secondary)]">
                {t("backupPage.dialog.allStacksCount", "{{count}} stacks", {
                  count: stacks.length,
                })}
              </code>
            </label>
          )}
          {stacks.map((s) => (
            <label key={s.id} className="flex cursor-pointer items-center gap-2 text-sm">
              <input
                type="radio"
                className="h-4 w-4 accent-[var(--color-primary)]"
                name="backup-stack"
                value={s.id}
                checked={stackId === s.id}
                onChange={() => chooseStack(s.id)}
              />
              <span>{s.name}</span>
              <code className="text-xs text-[var(--color-text-secondary)]">{s.namespace}</code>
            </label>
          ))}
          <label className="flex cursor-pointer items-center gap-2 text-sm">
            <input
              type="radio"
              className="h-4 w-4 accent-[var(--color-primary)]"
              name="backup-stack"
              value=""
              checked={stackId === ""}
              onChange={() => chooseStack("")}
            />
            <span>{t("backupPage.dialog.platformOnly", "Platform only (no stack)")}</span>
          </label>
        </fieldset>

        {all && (
          <p className="m-0 text-xs text-[var(--color-text-secondary)]">
            {t(
              "backupPage.dialog.allStacksHint",
              "One run for the platform and one per stack, tied together as a set — every target, every time. Restore a set from the API with its set ID.",
            )}
          </p>
        )}

        <ul className="flex flex-col gap-2">
          {TARGETS.map((target) => (
            <li key={target.id} className="flex gap-2">
              <Checkbox
                id={`backup-target-${target.id}`}
                align="start"
                checked={scope.includes(target.id)}
                // 전체는 언제나 전부 뜬다 — 서버가 세트의 범위를 정하므로 고를 수 없다.
                disabled={all || (!stack && STACK_TARGETS.has(target.id))}
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

        {!stack && !all && (
          <p className="m-0 text-xs text-[var(--color-text-secondary)]">
            {t(
              "backupPage.dialog.platformOnlyHint",
              "Vault, Kubernetes resources and volumes belong to a stack. Pick a stack to include them.",
            )}
          </p>
        )}

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
                {all
                  ? t(
                      "backupPage.dialog.allStacksWarning",
                      "Every stack's workloads stop in turn while its volumes are backed up.",
                    )
                  : t(
                      "backupPage.dialog.quiesceWarning",
                      "Including volumes stops the workloads while the backup runs.",
                    )}
              </span>
              <label className="text-xs" htmlFor="backup-confirm">
                {all
                  ? t("backupPage.dialog.allConfirmLabel", "Type the word to confirm")
                  : t(
                      "backupPage.dialog.confirmLabel",
                      "Type the namespace to confirm",
                    )}
                : <code>{confirmPhrase}</code>
              </label>
              <TextInput
                id="backup-confirm"
                value={confirm}
                onChange={(e) => setConfirm(e.target.value)}
                placeholder={confirmPhrase}
              />
            </div>
          </div>
        )}
      </div>
    </Modal>
  );
}
