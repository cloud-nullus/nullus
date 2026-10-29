import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Modal } from "../../../components/ui/modal";
import { Button } from "../../../components/ui/button";
import { Input } from "../../../components/ui/input";
import { Select } from "../../../components/ui/select";
import { Checkbox } from "../../../components/ui/checkbox";
import { useRegisterUpgradeBundle } from "../../stack/api/stack-api";
import type {
  RegisterUpgradeBundleInput,
  UpgradeBundle,
  UpgradeRiskLevel,
} from "../../stack/api/stack-api-types";

// 업그레이드 번들 등록 대화 상자(OSS 업그레이드 설계 §12.4). 번들은 실클러스터 리허설을 거친 경로다 —
// verified 로 넣으려면 "리허설했다" 확인이 있어야 하고, 없으면 draft 로만 들어가 후보에 보이지 않는다.
// 깊은 검사(대상이 출발보다 높은가, 저장소 주소, CVE 형식)는 서버가 한다. 여기서는 빈칸과 모양만 본다.

export const UPGRADE_TOOL_PRESETS: Record<
  string,
  { label: string; release_name: string; chart_name: string; repo_url: string }
> = {
  argocd: {
    label: "Argo CD",
    release_name: "argo-cd",
    chart_name: "argo-cd",
    repo_url: "https://argoproj.github.io/argo-helm",
  },
};

const VERSION_PATTERN = /^v?\d+(\.\d+){0,2}([-+][0-9A-Za-z.-]+)?$/;
const CVE_PATTERN = /^CVE-\d{4}-\d{4,}$/;

/** 쉼표 · 공백으로 나눈 CVE 목록. 대문자로 맞추고 중복을 뺀다. */
export function parseCVEList(raw: string): string[] {
  const out: string[] = [];
  for (const part of raw.split(/[\s,;]+/)) {
    const id = part.trim().toUpperCase();
    if (id && !out.includes(id)) out.push(id);
  }
  return out;
}

interface FormState {
  tool: string;
  release_name: string;
  chart_name: string;
  repo_url: string;
  source_chart_version: string;
  source_app_version: string;
  target_chart_version: string;
  target_app_version: string;
  risk_level: UpgradeRiskLevel;
  expected_disruption: string;
  release_notes: string;
  requires_backup: boolean;
  amd64: boolean;
  arm64: boolean;
  fixes_cves: string;
  status: "draft" | "verified";
  rehearsal_confirmed: boolean;
}

type FieldErrors = Partial<Record<keyof FormState, string>>;

function initialForm(tool = "argocd"): FormState {
  const preset = UPGRADE_TOOL_PRESETS[tool] ?? {
    label: tool,
    release_name: "",
    chart_name: "",
    repo_url: "",
  };
  return {
    tool,
    release_name: preset.release_name,
    chart_name: preset.chart_name,
    repo_url: preset.repo_url,
    source_chart_version: "",
    source_app_version: "",
    target_chart_version: "",
    target_app_version: "",
    risk_level: "low",
    expected_disruption: "",
    release_notes: "",
    requires_backup: true,
    amd64: true,
    arm64: true,
    fixes_cves: "",
    status: "verified",
    rehearsal_confirmed: false,
  };
}

export function validateBundleForm(
  form: FormState,
  t: (key: string) => string,
): FieldErrors {
  const errors: FieldErrors = {};
  const required: (keyof FormState)[] = [
    "release_name",
    "chart_name",
    "repo_url",
    "source_chart_version",
    "target_chart_version",
  ];
  for (const key of required) {
    if (String(form[key]).trim() === "")
      errors[key] = t("stackVersionsAdmin.bundles.register.required");
  }
  for (const key of ["source_chart_version", "target_chart_version"] as const) {
    const value = form[key].trim();
    if (value && !VERSION_PATTERN.test(value))
      errors[key] = t("stackVersionsAdmin.bundles.register.versionFormat");
  }
  if (
    !errors.target_chart_version &&
    form.target_chart_version.trim() !== "" &&
    form.target_chart_version.trim() === form.source_chart_version.trim()
  ) {
    errors.target_chart_version = t(
      "stackVersionsAdmin.bundles.register.targetSameAsSource",
    );
  }
  const badCVE = parseCVEList(form.fixes_cves).find(
    (id) => !CVE_PATTERN.test(id),
  );
  if (badCVE)
    errors.fixes_cves = t("stackVersionsAdmin.bundles.register.cveFormat");
  if (form.status === "verified" && !form.rehearsal_confirmed) {
    errors.rehearsal_confirmed = t(
      "stackVersionsAdmin.bundles.rehearsal.required",
    );
  }
  return errors;
}

export function toRegisterInput(form: FormState): RegisterUpgradeBundleInput {
  const archs = [form.amd64 ? "amd64" : "", form.arm64 ? "arm64" : ""].filter(
    Boolean,
  );
  return {
    tool: form.tool,
    release_name: form.release_name.trim(),
    chart_name: form.chart_name.trim(),
    repo_url: form.repo_url.trim(),
    source_chart_version: form.source_chart_version.trim(),
    source_app_version: form.source_app_version.trim(),
    target_chart_version: form.target_chart_version.trim(),
    target_app_version: form.target_app_version.trim(),
    risk_level: form.risk_level,
    expected_disruption: form.expected_disruption.trim(),
    release_notes: form.release_notes.trim(),
    requires_backup: form.requires_backup,
    supported_architectures: archs,
    fixes_cves: parseCVEList(form.fixes_cves),
    status: form.status,
    rehearsal_confirmed:
      form.status === "verified" ? form.rehearsal_confirmed : false,
  };
}

export function apiErrorMessage(error: unknown): string {
  const e = error as {
    response?: { data?: { error?: { message?: string } } };
    message?: string;
  } | null;
  return e?.response?.data?.error?.message || e?.message || "";
}

interface Props {
  open: boolean;
  onClose: () => void;
  onSaved?: (bundle: UpgradeBundle) => void;
}

export function UpgradeBundleRegisterModal({ open, onClose, onSaved }: Props) {
  const { t } = useTranslation();
  const register = useRegisterUpgradeBundle();
  const [form, setForm] = useState<FormState>(() => initialForm());
  const [errors, setErrors] = useState<FieldErrors>({});
  const [serverError, setServerError] = useState("");

  const set = <K extends keyof FormState>(key: K, value: FormState[K]) => {
    setForm((prev) => ({ ...prev, [key]: value }));
    setErrors((prev) => (prev[key] ? { ...prev, [key]: undefined } : prev));
  };

  const changeTool = (tool: string) => {
    const preset = UPGRADE_TOOL_PRESETS[tool];
    setForm((prev) => ({
      ...prev,
      tool,
      release_name: preset?.release_name ?? prev.release_name,
      chart_name: preset?.chart_name ?? prev.chart_name,
      repo_url: preset?.repo_url ?? prev.repo_url,
    }));
  };

  const handleSubmit = async () => {
    const found = validateBundleForm(form, (key) => t(key));
    setErrors(found);
    if (Object.values(found).some(Boolean)) return;
    setServerError("");
    try {
      const bundle = await register.mutateAsync(toRegisterInput(form));
      toast.success(t("stackVersionsAdmin.bundles.register.done"));
      onSaved?.(bundle);
      onClose();
    } catch (error) {
      setServerError(
        apiErrorMessage(error) ||
          t("stackVersionsAdmin.bundles.register.failed"),
      );
    }
  };

  const r = (key: string) => t(`stackVersionsAdmin.bundles.register.${key}`);

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={r("title")}
      size="wide"
      footer={
        <>
          <Button variant="outline" onClick={onClose}>
            {t("common.cancel", "Cancel")}
          </Button>
          <Button
            variant="primary"
            loading={register.isPending}
            onClick={() => void handleSubmit()}
          >
            {r("submit")}
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-4 text-[13px]">
        <p className="m-0 text-[var(--color-text-secondary)]">{r("intro")}</p>

        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
          <Select
            label={r("tool")}
            value={form.tool}
            onChange={(event) => changeTool(event.target.value)}
          >
            {Object.entries(UPGRADE_TOOL_PRESETS).map(([id, preset]) => (
              <option key={id} value={id}>
                {preset.label}
              </option>
            ))}
          </Select>
          <Select
            label={r("risk")}
            value={form.risk_level}
            onChange={(event) =>
              set("risk_level", event.target.value as UpgradeRiskLevel)
            }
          >
            <option value="low">{t("stackList.upgrade.risk.low")}</option>
            <option value="medium">{t("stackList.upgrade.risk.medium")}</option>
            <option value="high">{t("stackList.upgrade.risk.high")}</option>
          </Select>
          <Input
            label={r("releaseName")}
            value={form.release_name}
            error={errors.release_name}
            onChange={(e) => set("release_name", e.target.value)}
          />
          <Input
            label={r("chartName")}
            value={form.chart_name}
            error={errors.chart_name}
            onChange={(e) => set("chart_name", e.target.value)}
          />
          <div className="md:col-span-2">
            <Input
              label={r("repoUrl")}
              value={form.repo_url}
              error={errors.repo_url}
              onChange={(e) => set("repo_url", e.target.value)}
            />
          </div>
          <Input
            label={r("sourceChart")}
            placeholder="7.7.16"
            value={form.source_chart_version}
            error={errors.source_chart_version}
            onChange={(e) => set("source_chart_version", e.target.value)}
          />
          <Input
            label={r("sourceApp")}
            placeholder="v2.13.3"
            value={form.source_app_version}
            onChange={(e) => set("source_app_version", e.target.value)}
          />
          <Input
            label={r("targetChart")}
            placeholder="7.7.22"
            value={form.target_chart_version}
            error={errors.target_chart_version}
            onChange={(e) => set("target_chart_version", e.target.value)}
          />
          <Input
            label={r("targetApp")}
            placeholder="v2.13.4"
            value={form.target_app_version}
            helperText={r("targetAppHint")}
            onChange={(e) => set("target_app_version", e.target.value)}
          />
          <div className="md:col-span-2">
            <Input
              label={r("disruption")}
              value={form.expected_disruption}
              onChange={(e) => set("expected_disruption", e.target.value)}
            />
          </div>
          <div className="md:col-span-2">
            <Input
              label={r("releaseNotes")}
              value={form.release_notes}
              onChange={(e) => set("release_notes", e.target.value)}
            />
          </div>
          <div className="md:col-span-2">
            <Input
              label={r("fixesCves")}
              value={form.fixes_cves}
              error={errors.fixes_cves}
              helperText={errors.fixes_cves ? undefined : r("fixesCvesHint")}
              onChange={(e) => set("fixes_cves", e.target.value)}
            />
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-x-6 gap-y-2">
          <span className="text-[12px] font-semibold text-[var(--color-text-secondary)]">
            {r("architectures")}
          </span>
          <label className="flex items-center gap-2">
            <Checkbox
              checked={form.amd64}
              onChange={(e) => set("amd64", e.target.checked)}
            />
            amd64
          </label>
          <label className="flex items-center gap-2">
            <Checkbox
              checked={form.arm64}
              onChange={(e) => set("arm64", e.target.checked)}
            />
            arm64
          </label>
        </div>

        <label className="flex items-start gap-2">
          <Checkbox
            align="start"
            checked={form.requires_backup}
            onChange={(e) => set("requires_backup", e.target.checked)}
          />
          <span>
            <span className="text-[var(--color-text-primary)]">
              {r("requiresBackup")}
            </span>
            <span className="block text-[12px] text-[var(--color-text-secondary)]">
              {r("requiresBackupHint")}
            </span>
          </span>
        </label>

        <div className="rounded-lg border border-[var(--color-border-default)] p-3">
          <Select
            label={r("status")}
            value={form.status}
            onChange={(event) =>
              set("status", event.target.value as FormState["status"])
            }
          >
            <option value="verified">{r("statusVerified")}</option>
            <option value="draft">{r("statusDraft")}</option>
          </Select>
          {form.status === "verified" && (
            <label className="mt-3 flex items-start gap-2">
              <Checkbox
                align="start"
                checked={form.rehearsal_confirmed}
                onChange={(e) => set("rehearsal_confirmed", e.target.checked)}
                aria-invalid={!!errors.rehearsal_confirmed}
                aria-label={t("stackVersionsAdmin.bundles.rehearsal.label")}
              />
              <span>
                <span className="text-[var(--color-text-primary)]">
                  {t("stackVersionsAdmin.bundles.rehearsal.label")}
                </span>
                <span className="block text-[12px] text-[var(--color-text-secondary)]">
                  {t("stackVersionsAdmin.bundles.rehearsal.hint")}
                </span>
                {errors.rehearsal_confirmed && (
                  <span
                    className="block text-[12px] text-[var(--color-error)]"
                    role="alert"
                  >
                    {errors.rehearsal_confirmed}
                  </span>
                )}
              </span>
            </label>
          )}
        </div>

        {serverError && (
          <div className="text-[var(--color-error)]" role="alert">
            {serverError}
          </div>
        )}
      </div>
    </Modal>
  );
}
