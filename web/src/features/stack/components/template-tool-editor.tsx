import { useTranslation } from "react-i18next";
import { Check, Trash2 } from "lucide-react";

import { iconProps } from "../../../components/ui/icon";
import { Button } from "../../../components/ui/button";
import { Tabs } from "../../../components/ui/tabs";
import { TextInput } from "../../../components/ui/text-input";
import { cn } from "../../../lib/utils";
import { resolveToolIdByName } from "../utils/template-overrides";
import {
  ARTIFACTS_OPTIONS,
  AUTHENTICATION_OPTIONS,
  LOGGING_OPTIONS,
  MONITORING_OPTIONS,
  PIPELINE_OPTIONS,
} from "../utils/install-constants";
import type {
  AddToolDraft,
  ToolCategoryDefinition,
  ToolEntry,
  ToolSectionDefinition,
} from "../utils/template-config";

/**
 * 템플릿 편집 모달의 도구 선택 영역.
 *
 * 설치 마법사(stack-install-page)와 같은 구조를 쓴다 — 단계가 탭으로 갈리고,
 * 탭 안에서 카테고리별로 고를 수 있는 도구가 세로 카드로 펼쳐진다. 예전에는
 * 섹션이 아코디언으로 전부 세로로 쌓이고 한 줄짜리 6열 그리드가 그 안에
 * 들어 있었다. 카테고리 칸이 0.7fr 고정이라 "Package Registry" 같은 이름이
 * 접히거나 잘렸고, 고를 수 있는 도구는 좁은 `<select>` 안에 갇혀 열어 보기
 * 전에는 무엇이 있는지 알 수 없었다. 같은 일을 하는 두 화면이 서로 다르게
 * 생길 이유가 없다.
 *
 * 마법사와 다른 점은 **버전**이다. 마법사는 고르면 끝이지만 템플릿은 helm/app
 * 버전을 함께 pin 해야 하므로, 그 둘은 카드 아래 '상세' 줄로 내리고 적용은
 * 명시적인 버튼으로 남긴다 — 관리자가 버전을 타이핑하는 도중에 저장이 일어나면
 * 안 된다.
 */

const INSTALL_OPTIONS = [
  ...Object.values(ARTIFACTS_OPTIONS).flat(),
  ...Object.values(PIPELINE_OPTIONS).flat(),
  ...Object.values(MONITORING_OPTIONS).flat(),
  ...Object.values(LOGGING_OPTIONS).flat(),
  ...AUTHENTICATION_OPTIONS,
];

const OPTION_BY_LABEL = new Map(INSTALL_OPTIONS.map((option) => [option.label, option] as const));

/**
 * 한 도구 id 가 역할마다 다른 이름으로 나오는 경우를 걸러 낸다.
 *
 * 설명 문구는 `stackAddTools.tools.<id>` 하나뿐인데 GitLab 은 소스 저장소이자
 * 패키지 레지스트리다. id 로만 찾으면 "GitLab Package Registry" 밑에 "GitLab
 * source code management" 가 붙는다 — 실제로 그렇게 떴다. 이름이 하나로
 * 정해지는 id 만 id 로 찾고, 나머지는 이름이 정확히 맞을 때만 쓴다.
 */
const UNAMBIGUOUS_ID_LABEL = new Map<string, string>();
for (const option of INSTALL_OPTIONS) {
  const seen = UNAMBIGUOUS_ID_LABEL.get(option.id);
  if (seen === undefined) UNAMBIGUOUS_ID_LABEL.set(option.id, option.label);
  else if (seen !== option.label) UNAMBIGUOUS_ID_LABEL.set(option.id, "");
}

/**
 * 템플릿이 쓰는 제품명에 붙일 설명. 확실하지 않으면 아무것도 돌려주지 않는다.
 *
 * 이름이 정확히 맞아도 그것만으로는 부족하다 — 설명 문구는 `t()` 로 읽는데 그
 * 키가 `stackAddTools.tools.<id>` 라서 id 가 모호하면 번역이 있는 한 언제나
 * 엉뚱한 쪽이 이긴다(옵션에 적힌 문구는 fallback 이라 무시된다). 그래서 판단
 * 기준은 이름이 아니라 **id 가 한 제품만 가리키는가** 다.
 */
function toolCopyFor(optionName: string) {
  const soleLabel = UNAMBIGUOUS_ID_LABEL.get(resolveToolIdByName(optionName));
  if (!soleLabel) return undefined;
  return OPTION_BY_LABEL.get(soleLabel);
}

export interface TemplateToolSectionsProps {
  sections: ToolSectionDefinition[];
  activeSectionId: string;
  onSelectSection: (sectionId: string) => void;
  getCategories: (section: ToolSectionDefinition) => ToolCategoryDefinition[];
  tools: ToolEntry[];
  drafts: Record<string, AddToolDraft>;
  draftKeyFor: (sectionId: string, category: string) => string;
  defaultVersionsFor: (toolName: string) => { helm_version: string; app_version: string };
  onDraftChange: (draftKey: string, patch: Partial<AddToolDraft>) => void;
  onSubmitTool: (draftKey: string) => void;
  onRemoveTool: (sectionId: string, category: string) => void;
  onRemoveSection: (sectionId: string) => void;
}

export function TemplateToolSections({
  sections,
  activeSectionId,
  onSelectSection,
  getCategories,
  tools,
  drafts,
  draftKeyFor,
  defaultVersionsFor,
  onDraftChange,
  onSubmitTool,
  onRemoveTool,
  onRemoveSection,
}: TemplateToolSectionsProps) {
  const { t } = useTranslation();

  if (sections.length === 0) return null;

  // 지워진 섹션이 활성일 수 있으므로 실재하는 것으로 떨어뜨린다.
  const active = sections.find((s) => s.id === activeSectionId) ?? sections[0];

  return (
    <div className="rounded-lg border border-[var(--color-border-default)]">
      <Tabs
        items={sections.map((section) => ({ id: section.id, label: section.label }))}
        value={active.id}
        onChange={onSelectSection}
        className="px-2"
        trailing={
          <button
            type="button"
            onClick={() => onRemoveSection(active.id)}
            title={t("stackTemplatePage.actions.removeSection", "Remove {{section}} section", {
              section: active.label,
            })}
            className="mr-2 flex h-7 w-7 cursor-pointer items-center justify-center rounded border border-transparent text-[var(--color-text-muted)] transition-colors hover:border-[color-mix(in_srgb,_var(--color-error)_50%,_transparent)] hover:text-[var(--color-error)]"
          >
            <Trash2 {...iconProps("xs")} />
          </button>
        }
      />

      <div className="flex flex-col gap-5 p-3">
        {getCategories(active).map((category) => {
          const draftKey = draftKeyFor(active.id, category.category);
          const draft = drafts[draftKey];
          const applied = tools.find((tool) => tool.category === category.category);
          // draft 가 먼저다. 예전에는 적용된 값(applied)이 항상 이겨서, 이미
          // 템플릿에 담긴 카테고리는 다른 도구를 눌러도 선택이 움직이지 않았다.
          // 편집 모달을 열 때 draft 를 템플릿 값으로 심어 두므로 초기 선택은
          // 그대로 유지된다.
          const name = draft?.name || applied?.name || category.options[0] || "";
          const defaults = defaultVersionsFor(name);
          const helmVersion = draft?.helm_version ?? applied?.helm_version ?? defaults.helm_version;
          const appVersion = draft?.app_version ?? applied?.app_version ?? defaults.app_version;

          return (
            <TemplateToolCategory
              key={draftKey}
              category={category}
              name={name}
              helmVersion={helmVersion}
              appVersion={appVersion}
              applied={!!applied}
              onSelectName={(nextName) => {
                // 도구를 바꾸면 버전 기본값도 그 도구의 것으로 따라간다. 앞
                // 도구의 버전이 남으면 존재하지 않는 조합이 그대로 저장된다.
                onDraftChange(draftKey, {
                  category: category.category,
                  name: nextName,
                  ...defaultVersionsFor(nextName),
                });
              }}
              onVersionChange={(versions) =>
                onDraftChange(draftKey, { category: category.category, name, ...versions })
              }
              onSubmit={() => onSubmitTool(draftKey)}
              onRemove={() => onRemoveTool(active.id, category.category)}
            />
          );
        })}
      </div>
    </div>
  );
}

interface TemplateToolCategoryProps {
  category: ToolCategoryDefinition;
  name: string;
  helmVersion: string;
  appVersion: string;
  applied: boolean;
  onSelectName: (name: string) => void;
  onVersionChange: (versions: { helm_version: string; app_version: string }) => void;
  onSubmit: () => void;
  onRemove: () => void;
}

function TemplateToolCategory({
  category,
  name,
  helmVersion,
  appVersion,
  applied,
  onSelectName,
  onVersionChange,
  onSubmit,
  onRemove,
}: TemplateToolCategoryProps) {
  const { t } = useTranslation();

  return (
    <div className="flex flex-col gap-2.5">
      <div className="flex items-center gap-2">
        {/* 카테고리 이름은 제 줄을 갖는다 — 좁은 칸에 밀어 넣으면 잘린다. */}
        <span className="text-xs font-semibold uppercase tracking-[0.06em] text-[var(--color-text-secondary)]">
          {category.label}
        </span>
        {applied && (
          <span className="inline-flex items-center gap-1 rounded-full bg-[color-mix(in_srgb,_var(--color-success)_14%,_transparent)] px-2 py-0.5 text-[10px] font-semibold text-[var(--color-success)]">
            <Check {...iconProps("xs")} />
            {t("stackTemplatePage.form.toolApplied", "In template")}
          </span>
        )}
      </div>

      <div role="radiogroup" aria-label={category.label} className="flex flex-col gap-2">
        {category.options.map((option) => {
          const selected = option === name;
          const copy = toolCopyFor(option);
          return (
            <button
              key={option}
              type="button"
              role="radio"
              aria-checked={selected}
              onClick={() => onSelectName(option)}
              className={cn(
                "flex w-full cursor-pointer items-center gap-3 rounded-lg border px-[14px] py-2.5 text-left transition-colors duration-150",
                selected
                  ? "border-[color-mix(in_srgb,_var(--color-primary)_50%,_transparent)] bg-[color-mix(in_srgb,_var(--color-primary)_10%,_transparent)]"
                  : "border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)] hover:border-[var(--color-border-hover)]",
              )}
            >
              <span
                className={cn(
                  "flex h-4 w-4 shrink-0 items-center justify-center rounded-full border-2",
                  selected
                    ? "border-[var(--color-primary)] bg-[var(--color-primary)]"
                    : "border-[var(--color-border-hover)] bg-transparent",
                )}
              >
                {selected && <span className="h-1.5 w-1.5 rounded-full bg-white" />}
              </span>
              <span className="min-w-0">
                <span
                  className={cn(
                    "block text-sm font-semibold",
                    selected ? "text-[var(--color-primary)]" : "text-[var(--color-text-primary)]",
                  )}
                >
                  {/* 템플릿은 제품명을 그대로 들고 있다. 마법사의 라벨로 갈아
                      끼우면 저장되는 값과 화면의 글자가 어긋난다. */}
                  {option}
                </span>
                {copy && (
                  <span className="block text-xs text-[var(--color-text-secondary)]">
                    {t(`stackAddTools.tools.${copy.id}.description`, copy.description)}
                  </span>
                )}
              </span>
            </button>
          );
        })}
      </div>

      {/* 상세 — 버전 pin 과 적용. 마법사에는 없는 부분이다. */}
      <div className="grid gap-2 rounded-lg border border-dashed border-[var(--color-border-default)] p-2.5 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto_auto]">
        {/* 예전에는 placeholder 만 있어 값을 채우는 순간 두 상자가 무엇인지
            사라졌다. 어느 쪽이 차트 버전이고 어느 쪽이 앱 버전인지는 값만
            보고는 알 수 없다 — 라벨을 남겨 둔다. */}
        <VersionField
          label={t("stackTemplatePage.form.helmVersion", "Helm version")}
          placeholder={t("stackTemplatePage.form.helmVersionPlaceholder", "Helm version")}
          value={helmVersion}
          onChange={(next) => onVersionChange({ helm_version: next, app_version: appVersion })}
        />
        <VersionField
          label={t("stackTemplatePage.form.appVersion", "App version")}
          placeholder={t("stackTemplatePage.form.appVersionPlaceholder", "App version")}
          value={appVersion}
          onChange={(next) => onVersionChange({ helm_version: helmVersion, app_version: next })}
        />
        <Button variant="outline" size="sm" type="button" onClick={onSubmit} className="sm:self-end">
          {applied
            ? t("stackTemplatePage.actions.updateTool", "Update Tool")
            : t("stackTemplatePage.actions.addTool", "Add Tool")}
        </Button>
        <button
          type="button"
          aria-label={t("stackTemplatePage.actions.removeCategoryTool", "Remove {{category}}", {
            category: category.label,
          })}
          onClick={onRemove}
          className="flex h-[var(--control-height)] w-[var(--control-height)] cursor-pointer items-center justify-center rounded-lg border border-[var(--color-border-default)] text-[var(--color-text-secondary)] transition-colors hover:border-[color-mix(in_srgb,_var(--color-error)_50%,_transparent)] hover:text-[var(--color-error)] sm:self-end"
        >
          <Trash2 {...iconProps("xs")} />
        </button>
      </div>
    </div>
  );
}

/** 라벨이 붙은 버전 입력 한 칸. label 로 감싸 이름과 상자를 묶는다. */
function VersionField({
  label,
  placeholder,
  value,
  onChange,
}: {
  label: string;
  placeholder: string;
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <label className="flex min-w-0 flex-col gap-1">
      <span className="text-[11px] text-[var(--color-text-secondary)]">{label}</span>
      <TextInput
        type="text"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
      />
    </label>
  );
}
