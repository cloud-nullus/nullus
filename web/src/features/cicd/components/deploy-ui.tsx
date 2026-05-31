import { useState } from "react";
import { Check, Loader2, Copy } from "lucide-react";
import { Input } from "../../../components/ui/input";
import { cn } from "../../../lib/utils";

export const labelStyleClass =
  "mb-1.5 block text-xs font-semibold uppercase tracking-[0.04em] text-[var(--color-text-secondary)]";

export function PhaseStep({
  label,
  index,
  progress,
  total,
}: {
  label: string;
  index: number;
  progress: number;
  total: number;
}) {
  const phaseProgress = 100 / total;
  const phaseStart = index * phaseProgress;
  const isDone = progress >= phaseStart + phaseProgress;
  const isActive = progress >= phaseStart && !isDone;

  return (
    <div className="flex flex-1 items-center gap-2">
      <div
        className={cn(
          "flex h-7 w-7 shrink-0 items-center justify-center rounded-full transition-all duration-300",
          isDone
            ? "bg-[rgba(34,197,94,0.15)] text-[#22c55e]"
            : isActive
              ? "bg-[rgba(99,102,241,0.15)] text-[#818cf8]"
              : "bg-[rgba(255,255,255,0.05)] text-[var(--color-text-secondary)]",
        )}
      >
        {isDone ? (
          <Check size={14} />
        ) : isActive ? (
          <Loader2 size={14} className="animate-spin" />
        ) : (
          <span className="text-xs font-bold">{index + 1}</span>
        )}
      </div>
      <span
        className={cn(
          "text-[13px] font-semibold",
          isDone
            ? "text-[#22c55e]"
            : isActive
              ? "text-[#a5b4fc]"
              : "text-[var(--color-text-secondary)]",
        )}
      >
        {label}
      </span>
      {index < total - 1 && (
        <div
          className={cn(
            "mx-1 h-px flex-1 transition-colors duration-300",
            isDone
              ? "bg-[rgba(34,197,94,0.4)]"
              : "bg-[var(--color-border-default)]",
          )}
        />
      )}
    </div>
  );
}

export function StepSection({
  title,
  children,
}: {
  title: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <div>
      <p className="mb-4 mt-0 flex items-center gap-2 text-[15px] font-bold text-[var(--color-text-primary)]">
        {title}
      </p>
      {children}
    </div>
  );
}

export function ResourceSlider({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: string;
  options: string[];
  onChange: (v: string) => void;
}) {
  const idx = options.indexOf(value);
  const isCustom = idx === -1;
  const sliderId = `resource-${label.toLowerCase().replace(/\s+/g, "-")}`;
  return (
    <div>
      <div className="mb-1.5 flex items-center justify-between">
        <label htmlFor={sliderId} className={cn(labelStyleClass, "mb-0")}>
          {label}
        </label>
        <Input
          value={value}
          onChange={(e) => onChange(e.target.value)}
          className="w-24 text-right font-mono text-[13px]"
        />
      </div>
      <input
        id={sliderId}
        type="range"
        min={0}
        max={options.length - 1}
        value={isCustom ? 0 : idx}
        onChange={(e) => onChange(options[Number(e.target.value)])}
        className="w-full accent-[#6366f1]"
      />
      <div className="mt-1 flex justify-between">
        {options.map((o) => (
          <span
            key={o}
            className="font-mono text-[10px] text-[var(--color-text-secondary)]"
          >
            {o}
          </span>
        ))}
      </div>
    </div>
  );
}

export function CopyableCommand({ command }: { command: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <div className="mt-3 flex items-center gap-2 rounded-md bg-[#0d1117] px-3 py-2">
      <code className="flex-1 overflow-x-auto whitespace-nowrap font-mono text-xs text-[#c9d1d9]">
        <span className="mr-1.5 text-[#484f58]">$</span>
        {command}
      </code>
      <button
        type="button"
        onClick={() => {
          void navigator.clipboard.writeText(command);
          setCopied(true);
          setTimeout(() => setCopied(false), 2000);
        }}
        className="shrink-0 cursor-pointer border-none bg-none p-1 text-[rgba(255,255,255,0.4)] transition-colors hover:text-white"
      >
        {copied ? (
          <Check size={14} className="text-[#3fb950]" />
        ) : (
          <Copy size={14} />
        )}
      </button>
    </div>
  );
}
