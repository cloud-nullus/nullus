import { lazy, Suspense, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { FileCode2, RefreshCw } from "lucide-react";
import { toast } from "sonner";

import { Button } from "../../../components/ui/button";
import { Modal } from "../../../components/ui/modal";
import { iconProps } from "../../../components/ui/icon";
import { StatusIcon } from "../../../components/ui/status-icon";
import { cn } from "../../../lib/utils";
import type {
  ApplyReleaseValuesResponse,
  ReleaseValuesMode,
  StackRelease,
} from "../api/stack-api";
import {
  useApplyReleaseValues,
  usePreviewReleaseValues,
  useReleaseValues,
  useStackReleases,
} from "../api/stack-api";

// Monaco 는 무겁다. 스택 목록 화면이 이 탭을 정적으로 들고 있으면 설정을
// 편집할 생각이 없는 사용자도 에디터를 함께 내려받는다. 탭을 열 때 가져온다.
const MonacoYamlEditor = lazy(() =>
  import("../../../components/shared/monaco-yaml-editor").then((m) => ({
    default: m.MonacoYamlEditor,
  })),
);

/**
 * 배포된 스택의 OSS 설정을 values.yaml 수준에서 고치는 탭.
 * (기능분해도 NULLUS_DSS_040_040 — 스택 설정 수정 및 재배포)
 *
 * 편집 단위는 두 가지다. 전체 values 는 배포된 그대로라 직관적이지만 플랫폼이
 * 계산해 넣은 값까지 노출되고, 오버라이드는 안전한 대신 지금 무엇이 적용돼
 * 있는지 보이지 않는다. 어느 쪽이 맞는지는 상황마다 다르므로 고르게 한다.
 */
export function StackConfigTab({ stackId }: { stackId: string }) {
  const { t } = useTranslation();
  const [mode, setMode] = useState<ReleaseValuesMode>("live");
  const [selectedRelease, setSelectedRelease] = useState<string>("");
  const [draft, setDraft] = useState("");
  const [preview, setPreview] = useState<ApplyReleaseValuesResponse | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);

  const releasesQuery = useStackReleases(stackId);
  const releases = useMemo<StackRelease[]>(
    () => releasesQuery.data ?? [],
    [releasesQuery.data],
  );

  useEffect(() => {
    if (!selectedRelease && releases.length > 0) {
      setSelectedRelease(releases[0].release_name);
    }
  }, [releases, selectedRelease]);

  const valuesQuery = useReleaseValues(stackId, selectedRelease, mode);
  const previewMutation = usePreviewReleaseValues();
  const applyMutation = useApplyReleaseValues();

  // 서버에서 받은 원본으로 편집 버퍼를 초기화한다. 릴리스나 모드를 바꾸면
  // 다른 문서가 되므로 그때만 갈아끼운다 — 매 렌더마다 덮으면 타이핑이 지워진다.
  useEffect(() => {
    setDraft(valuesQuery.data?.yaml ?? "");
    setPreview(null);
  }, [valuesQuery.data?.yaml, valuesQuery.data?.mode, valuesQuery.data?.release_name]);

  const activeRelease = releases.find((r) => r.release_name === selectedRelease);
  const protectedPaths = valuesQuery.data?.protected_paths ?? [];
  const isBusy = previewMutation.isPending || applyMutation.isPending;

  const runPreview = async () => {
    try {
      const result = await previewMutation.mutateAsync({
        stackId,
        releaseName: selectedRelease,
        mode,
        yaml: draft,
      });
      setPreview(result);
    } catch (error) {
      setPreview(null);
      toast.error(extractApiMessage(error, t("stackConfig.previewFailed", "미리보기에 실패했습니다")));
    }
  };

  const runApply = async () => {
    setConfirmOpen(false);
    try {
      const result = await applyMutation.mutateAsync({
        stackId,
        releaseName: selectedRelease,
        mode,
        yaml: draft,
      });
      setPreview(result);
      toast.success(
        t("stackConfig.applied", {
          defaultValue: "설정을 적용했습니다 (revision {{revision}})",
          revision: result.revision,
        }),
      );
    } catch (error) {
      toast.error(extractApiMessage(error, t("stackConfig.applyFailed", "설정 적용에 실패했습니다")));
    }
  };

  if (releasesQuery.isLoading) {
    return (
      <p className="text-[13px] text-[var(--color-text-secondary)]">
        {t("stackConfig.loadingReleases", "배포된 릴리스를 읽는 중…")}
      </p>
    );
  }

  if (releasesQuery.isError) {
    return (
      <div className="rounded-[var(--card-radius)] border border-[color-mix(in_srgb,_var(--color-error)_35%,_transparent)] bg-[color-mix(in_srgb,_var(--color-error)_8%,_transparent)] p-4 text-[13px] text-[var(--color-error)]">
        {t("stackConfig.releasesFailed", "릴리스 목록을 읽지 못했습니다. 클러스터 연결을 확인하세요.")}
      </div>
    );
  }

  if (releases.length === 0) {
    return (
      <p className="text-[13px] text-[var(--color-text-secondary)]">
        {t("stackConfig.noReleases", "이 스택에는 편집할 Helm 릴리스가 없습니다.")}
      </p>
    );
  }

  return (
    // 탭이 상세 패널 높이를 그대로 채운다. 릴리스 목록이 가로로 깔려 있을 때는
    // 그것만으로 화면 절반을 먹어 에디터가 바깥 스크롤 아래로 밀려났다 —
    // 설정을 고치려면 매번 스크롤을 내려야 했다. 목록을 왼쪽 레일로 세우면
    // 세로를 되찾고, 남는 높이는 전부 에디터가 가져간다.
    <div className="flex h-full min-h-0 flex-col gap-3 lg:flex-row lg:gap-4">
      <aside className="flex min-h-0 shrink-0 flex-col gap-2 lg:w-[240px]">
        <div className="flex items-center justify-between gap-2">
          <span className="text-[12px] font-semibold text-[var(--color-text-secondary)]">
            {t("stackConfig.release", "릴리스")}
          </span>
          <Button
            variant="ghost"
            onClick={() => void releasesQuery.refetch()}
            aria-label={t("stackConfig.refresh", "릴리스 목록 새로고침")}
          >
            <RefreshCw {...iconProps("xs")} />
          </Button>
        </div>
        {/* 좁은 화면에서는 레일이 위로 올라오므로 여러 열로 눕히고 높이를
            묶는다 — 릴리스가 열댓 개라 한 줄씩 쌓으면 그것만으로 화면을 덮는다. */}
        {/* min-h-0 이 없으면 flex 자식이 제 콘텐츠 높이 아래로 줄지 못한다 —
            목록이 레일 밖으로 흘러 아래쪽 릴리스가 잘리고, 늘어난 만큼 오른쪽
            에디터가 최소 높이까지 밀린다. */}
        <ul className="m-0 grid min-h-0 list-none grid-cols-2 gap-1 overflow-y-auto p-0 max-lg:max-h-[132px] sm:grid-cols-3 lg:grid-cols-1 lg:pr-1">
          {releases.map((release) => {
            const isSelected = release.release_name === selectedRelease;
            return (
              <li key={release.release_name}>
                <button
                  type="button"
                  aria-pressed={isSelected}
                  onClick={() => setSelectedRelease(release.release_name)}
                  className={cn(
                    "flex w-full items-center gap-2 rounded-lg border px-3 py-2 text-left text-xs",
                    isSelected
                      ? "border-[color-mix(in_srgb,_var(--color-primary)_50%,_transparent)] bg-[color-mix(in_srgb,_var(--color-primary)_10%,_transparent)] text-[var(--color-primary)]"
                      : "border-[var(--color-border-default)] text-[var(--color-text-primary)]",
                  )}
                >
                  {/* 릴리스 이름은 자르지 않는다 — 잘리면 어느 OSS 인지가 사라진다.
                      레일 너비는 가장 긴 이름(kube-prometheus-stack)에 맞춰 두었다. */}
                  <span className="whitespace-nowrap font-semibold">{release.release_name}</span>
                  <span className="ml-auto shrink-0 rounded border border-[var(--color-border-default)] px-1.5 py-0.5 text-[10px] text-[var(--color-text-secondary)]">
                    rev {release.revision}
                  </span>
                </button>
              </li>
            );
          })}
        </ul>
      </aside>

      <div className="flex min-h-0 min-w-0 flex-1 flex-col gap-2.5">
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-[12px] font-semibold text-[var(--color-text-secondary)]">
            {t("stackConfig.mode", "편집 단위")}
          </span>
          {(["live", "override"] as const).map((candidate) => (
            <button
              key={candidate}
              type="button"
              aria-pressed={candidate === mode}
              onClick={() => setMode(candidate)}
              className={cn(
                "rounded-lg border px-3 py-[7px] text-xs font-semibold",
                candidate === mode
                  ? "border-[color-mix(in_srgb,_var(--color-primary)_50%,_transparent)] bg-[color-mix(in_srgb,_var(--color-primary)_10%,_transparent)] text-[var(--color-primary)]"
                  : "border-[var(--color-border-default)] text-[var(--color-text-primary)]",
              )}
            >
              {candidate === "live"
                ? t("stackConfig.modeLive", "전체 values (배포된 그대로)")
                : t("stackConfig.modeOverride", "내 오버라이드만")}
            </button>
          ))}
          <p className="m-0 basis-full text-[12px] text-[var(--color-text-secondary)]">
            {mode === "live"
              ? t(
                  "stackConfig.modeLiveHint",
                  "릴리스에 실제로 배포된 values 전체입니다. 여기서 키를 지우면 그 값도 함께 사라집니다.",
                )
              : t(
                  "stackConfig.modeOverrideHint",
                  "배포된 값 위에 얹을 커스텀만 적습니다. 이미 적용된 값은 오버라이드를 비워도 되돌아가지 않습니다 — 되돌리려면 전체 values 모드에서 해당 키를 지우세요.",
                )}
          </p>
        </div>

        {activeRelease && (
          <div className="rounded-lg border border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-primary)_8%,_transparent)] px-3 py-2 text-xs">
            <div className="flex flex-wrap items-center gap-2">
              <FileCode2 {...iconProps("xs")} />
              <span className="font-semibold text-[var(--color-primary)]">{activeRelease.release_name}</span>
              <span className="text-[var(--color-text-secondary)]">
                {activeRelease.chart_name}
                {activeRelease.chart_version ? ` ${activeRelease.chart_version}` : ""} · {activeRelease.status}
              </span>
            </div>
            {!activeRelease.step_name && (
              <div className="mt-1 text-[var(--color-warning)]">
                {t(
                  "stackConfig.unmappedRelease",
                  "설치 단계를 알 수 없는 릴리스입니다. 편집은 되지만 재배포 시 유지되지 않을 수 있습니다.",
                )}
              </div>
            )}
            {protectedPaths.length > 0 && (
              <div className="mt-1 text-[var(--color-text-secondary)]">
                {t("stackConfig.protectedPaths", "플랫폼이 관리하는 값")}: {protectedPaths.join(", ")}
              </div>
            )}
          </div>
        )}

        {valuesQuery.isLoading ? (
          <p className="text-[13px] text-[var(--color-text-secondary)]">
            {t("stackConfig.loadingValues", "values 를 읽는 중…")}
          </p>
        ) : (
          <Suspense
            fallback={
              <p className="text-[13px] text-[var(--color-text-secondary)]">
                {t("stackConfig.loadingEditor", "편집기를 불러오는 중…")}
              </p>
            }
          >
            {/* 남는 높이를 전부 가져가되 바닥은 둔다. 화면이 아주 낮으면
                에디터를 짜부라뜨리는 것보다 바깥이 스크롤되는 편이 낫다. */}
            <MonacoYamlEditor
              value={draft}
              onChange={setDraft}
              height="100%"
              className="min-h-[220px] flex-1"
              ariaLabel={t("stackConfig.editorLabel", "릴리스 values 편집기")}
            />
          </Suspense>
        )}

        <div className="flex shrink-0 flex-wrap items-center gap-2">
          <Button variant="secondary" onClick={() => void runPreview()} disabled={isBusy}>
            {t("stackConfig.preview", "변경 미리보기")}
          </Button>
          <Button onClick={() => setConfirmOpen(true)} disabled={isBusy}>
            {t("stackConfig.apply", "적용")}
          </Button>
          <Button
            variant="ghost"
            onClick={() => {
              setDraft(valuesQuery.data?.yaml ?? "");
              setPreview(null);
            }}
            disabled={isBusy}
          >
            {t("stackConfig.reset", "되돌리기")}
          </Button>
        </div>

        {/* 미리보기는 열려 있을 때만 자리를 차지하고, 길어지면 자기 안에서
            스크롤한다 — 여기서 늘어나면 에디터가 밀려 다시 바깥이 스크롤된다. */}
        {preview && (
          <div className="min-h-0 shrink-0 overflow-y-auto lg:max-h-[38%]">
            <PreviewPanel preview={preview} />
          </div>
        )}
      </div>

      <Modal
        open={confirmOpen}
        onClose={() => setConfirmOpen(false)}
        title={t("stackConfig.confirmTitle", "설정을 적용할까요?")}
      >
        <div className="flex flex-col gap-3 text-[13px]">
          <p className="m-0 text-[var(--color-text-secondary)]">
            {t("stackConfig.confirmBody", {
              defaultValue:
                "{{release}} 릴리스에 helm upgrade 가 실행됩니다. 설정에 따라 파드가 다시 시작되어 잠시 중단될 수 있습니다.",
              release: selectedRelease,
            })}
          </p>
          {(preview?.warnings?.length ?? 0) > 0 && (
            <div className="rounded border border-[color-mix(in_srgb,_var(--color-warning)_40%,_transparent)] bg-[color-mix(in_srgb,_var(--color-warning)_10%,_transparent)] p-2 text-[var(--color-warning)]">
              {t("stackConfig.confirmWarnings", {
                defaultValue: "플랫폼이 관리하는 값 {{count}}건을 건드렸습니다.",
                count: preview?.warnings?.length ?? 0,
              })}
            </div>
          )}
          <div className="flex justify-end gap-2">
            <Button variant="ghost" onClick={() => setConfirmOpen(false)}>
              {t("common.cancel", "취소")}
            </Button>
            <Button onClick={() => void runApply()} disabled={applyMutation.isPending}>
              {t("stackConfig.apply", "적용")}
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}

function PreviewPanel({ preview }: { preview: ApplyReleaseValuesResponse }) {
  const { t } = useTranslation();
  const warnings = preview.warnings ?? [];

  return (
    <div className="flex flex-col gap-3">
      {warnings.length > 0 && (
        <div className="rounded-[var(--card-radius)] border border-[color-mix(in_srgb,_var(--color-warning)_40%,_transparent)] bg-[color-mix(in_srgb,_var(--color-warning)_8%,_transparent)] p-3 text-xs">
          <div className="mb-1 flex items-center gap-2 font-semibold text-[var(--color-warning)]">
            <StatusIcon tone="warning" size="xs" inheritColor />
            {t("stackConfig.warningsTitle", "플랫폼이 관리하는 값을 건드렸습니다")}
          </div>
          <ul className="m-0 list-disc pl-5 text-[var(--color-text-secondary)]">
            {warnings.map((warning) => (
              <li key={`${warning.path}-${warning.kind}`}>{warning.message}</li>
            ))}
          </ul>
        </div>
      )}

      {preview.render_error && (
        <div className="rounded-[var(--card-radius)] border border-[color-mix(in_srgb,_var(--color-error)_35%,_transparent)] bg-[color-mix(in_srgb,_var(--color-error)_8%,_transparent)] p-3 text-xs">
          <div className="mb-1 flex items-center gap-2 font-semibold text-[var(--color-error)]">
            <StatusIcon tone="error" size="xs" inheritColor />
            {t("stackConfig.renderFailed", "이 값으로는 차트가 렌더되지 않습니다")}
          </div>
          <pre className="m-0 max-h-[160px] overflow-auto whitespace-pre-wrap text-[var(--color-text-secondary)]">
            {preview.render_error}
          </pre>
        </div>
      )}

      {preview.dry_run && !preview.render_error && warnings.length === 0 && (
        <div className="flex items-center gap-2 rounded-[var(--card-radius)] border border-[var(--color-border-default)] p-3 text-xs text-[var(--color-text-secondary)]">
          <StatusIcon tone="success" size="xs" />
          {t("stackConfig.previewClean", "플랫폼 관리 값은 그대로입니다. 렌더도 통과했습니다.")}
        </div>
      )}

      {preview.effective_yaml && (
        <div>
          <div className="mb-1 text-[12px] font-semibold text-[var(--color-text-secondary)]">
            {t("stackConfig.effectiveValues", "실제로 적용되는 values")}
          </div>
          <pre className="max-h-[280px] overflow-auto rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_3%,_transparent)] p-3 text-[12px]">
            {preview.effective_yaml}
          </pre>
        </div>
      )}
    </div>
  );
}

function extractApiMessage(error: unknown, fallback: string): string {
  const response = (error as { response?: { data?: { error?: { message?: string } } } })?.response;
  return response?.data?.error?.message ?? fallback;
}
