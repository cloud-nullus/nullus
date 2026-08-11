import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { cn } from "../../../lib/utils";
import { toolLogoURL } from "../../stack/utils/tool-logo";
import type { ToolHealth, ToolHealthStatus } from "../../../types";

// 색만으로 상태를 구분하면 색각 이상 사용자가 읽을 수 없다. 점 + 글자를 함께 쓴다.
const STATUS_STYLE: Record<ToolHealthStatus, { dot: string; text: string; chip: string }> = {
  running: {
    dot: "bg-[var(--color-success)]",
    text: "text-[var(--color-success)]",
    chip: "border-[color-mix(in srgb, var(--color-success) 35%, transparent)] bg-[color-mix(in srgb, var(--color-success) 8%, transparent)]",
  },
  warning: {
    dot: "bg-[var(--color-warning)]",
    text: "text-[var(--color-warning)]",
    chip: "border-[color-mix(in srgb, var(--color-warning) 35%, transparent)] bg-[color-mix(in srgb, var(--color-warning) 8%, transparent)]",
  },
  error: {
    dot: "bg-[var(--color-error)]",
    text: "text-[var(--color-error)]",
    chip: "border-[color-mix(in srgb, var(--color-error) 40%, transparent)] bg-[color-mix(in srgb, var(--color-error) 10%, transparent)]",
  },
};

// 문제가 있는 도구가 먼저 눈에 들어와야 한다.
const STATUS_ORDER: Record<ToolHealthStatus, number> = { error: 0, warning: 1, running: 2 };

function styleFor(status: ToolHealthStatus) {
  return STATUS_STYLE[status] ?? STATUS_STYLE.warning;
}

export function PlatformToolHealth({
  tools,
  isLoading,
}: {
  tools: ToolHealth[] | undefined;
  isLoading: boolean;
}) {
  const { t } = useTranslation();

  const sorted = useMemo(
    () =>
      [...(tools ?? [])].sort(
        (a, b) =>
          (STATUS_ORDER[a.status] ?? 1) - (STATUS_ORDER[b.status] ?? 1) ||
          a.name.localeCompare(b.name),
      ),
    [tools],
  );

  const healthy = sorted.filter((tool) => tool.status === "running").length;

  return (
    <div className="mb-6 rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-[var(--card-padding)]">
      <div className="mb-3.5 flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <h2 className="m-0 text-[15px] font-bold text-[var(--color-text-primary)]">
            {t("observability.platformToolHealth", "Platform Tool Health")}
          </h2>
          {isLoading && (
            <span className="rounded-full bg-[color-mix(in srgb, var(--color-primary) 15%, transparent)] px-2 py-0.5 text-[11px] font-semibold text-[var(--color-primary)]">
              {t("common.loading", "Loading...")}
            </span>
          )}
        </div>
        {sorted.length > 0 && (
          <span className="text-xs font-semibold text-[var(--color-text-secondary)]">
            {t("observability.platformToolHealthSummary", {
              healthy,
              total: sorted.length,
            })}
          </span>
        )}
      </div>

      {sorted.length === 0 ? (
        <p className="m-0 text-[13px] text-[var(--color-text-secondary)]">
          {isLoading
            ? t("observability.platformToolHealthLoading", "Checking installed tools...")
            : t(
                "observability.platformToolHealthEmpty",
                "No installed tools yet. Tools appear here once a stack finishes installing.",
              )}
        </p>
      ) : (
        <ul className="m-0 grid list-none grid-cols-[repeat(auto-fill,minmax(190px,1fr))] gap-2.5 p-0">
          {sorted.map((tool) => {
            const style = styleFor(tool.status);
            return (
              <li
                key={tool.name}
                aria-label={tool.name}
                className={cn(
                  "flex items-center gap-2.5 rounded-[10px] border px-3 py-2.5",
                  style.chip,
                )}
              >
                <img
                  src={toolLogoURL(tool.name)}
                  alt=""
                  aria-hidden="true"
                  loading="lazy"
                  className="h-5 w-5 shrink-0 rounded-[4px]"
                  onError={(e) => {
                    e.currentTarget.style.visibility = "hidden";
                  }}
                />
                <div className="min-w-0 flex-1">
                  <div className="truncate text-[13px] font-semibold text-[var(--color-text-primary)]">
                    {tool.name}
                  </div>
                  <div className="truncate text-[11px] text-[var(--color-text-secondary)]">
                    {tool.version}
                  </div>
                </div>
                <div className="flex shrink-0 items-center gap-1.5">
                  <span className={cn("h-2 w-2 rounded-full", style.dot)} />
                  <span className={cn("text-[11px] font-semibold", style.text)}>{tool.status}</span>
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
