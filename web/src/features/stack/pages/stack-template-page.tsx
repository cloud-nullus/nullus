import { useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { BookOpen, ExternalLink, Pencil, Plus, Trash2, Wrench } from 'lucide-react';
import { iconProps } from '../../../components/ui/icon'
import {
  useCreateTemplate,
  useDeleteTemplate,
  useTemplates,
  useUpdateTemplate,
} from "../api/stack-api";
import type { StackTemplate } from "../api/stack-api";
import { useStackConfigStore } from "../stores/stack-config-store";
import { Button } from "../../../components/ui/button";
import { Input } from "../../../components/ui/input";
import { Select } from "../../../components/ui/select";
import { Modal } from "../../../components/ui/modal";
import { ConfirmDialog } from "../../../components/shared/confirm-dialog";
import { useAuthStore } from "../../../stores/auth-store";
import { resolveLocale } from "../../../lib/locale";
import {
  DEFAULT_PLANNING_PROFILE,
  PLANNING_PROFILE_VALUES,
  type PlanningProfile,
} from "../../../types";
import { buildInstallOverridesFromTemplate } from "../utils/template-overrides";
import {
  type TemplateFormState,
  type ToolEntry,
  type ToolSectionDefinition,
  type AddToolDraft,
  TOOL_SECTIONS,
  TOOL_CATEGORY_LOOKUP,
  TEMPLATE_DESCRIPTION_I18N,
  TEMPLATE_DESCRIPTION_LOCALE_OVERRIDES,
  NS_PER_MINUTE,
  getSectionCategories,
  addToolDraftKey,
  buildInitialAddToolDrafts,
  toToolEntry,
  EMPTY_TEMPLATE_FORM,
  defaultVersionsForTool,
  toTemplateSlug,
  createTemplateUUID,
  estimateInstallMinutesFromTools,
  estimateInstallMinutesForTemplate,
} from "../utils/template-config";
import { PageHeader } from '../../../components/layout/page-header'
import { SearchInput } from "../../../components/ui/search-input"
import { TextInput } from "../../../components/ui/text-input"
import { TemplateToolSections } from "../components/template-tool-editor"

const TOOL_CATEGORY_COLORS: Record<string, { bg: string; color: string }> = {
  source_repository: { bg: "color-mix(in srgb, var(--color-info) 12%, transparent)", color: "var(--color-info)" },
  ci_platform: { bg: "color-mix(in srgb, var(--color-accent-alt) 12%, transparent)", color: "var(--color-accent-alt)" },
  container_registry: { bg: "color-mix(in srgb, var(--color-success) 12%, transparent)", color: "var(--color-success)" },
  storage_backend: { bg: "color-mix(in srgb, var(--color-warning) 12%, transparent)", color: "var(--color-warning)" },
  cd_tool: { bg: "color-mix(in srgb, var(--color-accent-alt) 12%, transparent)", color: "var(--color-accent-alt)" },
  monitoring_collection: { bg: "color-mix(in srgb, var(--color-accent-alt) 12%, transparent)", color: "var(--color-accent-alt)" },
  monitoring_visualization: { bg: "color-mix(in srgb, var(--color-info) 12%, transparent)", color: "var(--color-info)" },
  log_aggregation: { bg: "color-mix(in srgb, var(--color-warning) 12%, transparent)", color: "var(--color-warning)" },
};

const getToolCategoryColor = (category: string) =>
  TOOL_CATEGORY_COLORS[category] ?? {
    bg: "color-mix(in srgb, var(--color-primary) 12%, transparent)",
    color: "var(--color-primary)",
  };

/**
 * 표시용 도구 목록에서 중복을 걷어낸다.
 *
 * 서버가 주는 tools 는 "역할별 선택"의 나열이라 한 제품이 두 역할을 겸하면 두 번
 * 들어온다 — Nexus 는 컨테이너 레지스트리이면서 패키지 저장소다. 그대로 그리면
 * 칩이 두 개 뜨고("Nexus … Nexus"), key={tool} 이 충돌해 React 가 경고를 낸다.
 * 같은 판단이 utils/stack-list-utils.ts 의 도구 수 집계에도 이미 들어가 있다.
 */
function uniqueTools(tools: string[] | undefined): string[] {
  return Array.from(new Set(Array.isArray(tools) ? tools : []))
}

export function StackTemplatePage() {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const { data: apiTemplates } = useTemplates();
  const createTemplate = useCreateTemplate();
  const updateTemplate = useUpdateTemplate();
  const deleteTemplate = useDeleteTemplate();
  const role = useAuthStore((state) => state.role);
  const isAdmin = role === "admin";
  const { setTemplate, loadFromTemplate } = useStackConfigStore();
  const [search, setSearch] = useState("");
  const [selectedTemplateId, setSelectedTemplateId] = useState<string | null>(
    null,
  );
  const [formOpen, setFormOpen] = useState(false);
  const [editingTemplateId, setEditingTemplateId] = useState<string | null>(
    null,
  );
  const [deleteTemplateId, setDeleteTemplateId] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const [form, setForm] = useState<TemplateFormState>(EMPTY_TEMPLATE_FORM);
  const [customSections, setCustomSections] = useState<ToolSectionDefinition[]>(
    [],
  );
  const [removedBaseSectionIds, setRemovedBaseSectionIds] = useState<string[]>(
    [],
  );
  const [addSectionOpen, setAddSectionOpen] = useState(false);
  const [newSectionLabel, setNewSectionLabel] = useState("");
  const [newCategoryLabel, setNewCategoryLabel] = useState("");
  const [newCategoryOptions, setNewCategoryOptions] = useState("");

  // 어떤 섹션을 감출지는 모달을 열 때 한 번 정한다(openEditModal). 편집 중의
  // form.tools 로 매번 다시 고르면, 섹션의 마지막 도구를 지우는 순간 그 탭이
  // 사라져 방금 지운 것을 되돌릴 수 없다.
  const visibleBaseSections = useMemo(
    () =>
      TOOL_SECTIONS.filter(
        (section) => !removedBaseSectionIds.includes(section.id),
      ),
    [removedBaseSectionIds],
  );

  const allSections = [...visibleBaseSections, ...customSections];
  const estimatedInstallMinutes = useMemo(
    () => estimateInstallMinutesFromTools(form.tools),
    [form.tools],
  );
  const estimatedInstallTimeNs = estimatedInstallMinutes * NS_PER_MINUTE;

  const [addToolDrafts, setAddToolDrafts] = useState<
    Record<string, AddToolDraft>
  >(buildInitialAddToolDrafts);
  // 설치 마법사처럼 섹션을 탭으로 가른다. 지워진 섹션이 활성으로 남을 수 있어
  // 렌더 쪽에서 실재하는 것으로 떨어뜨린다.
  const [activeToolSectionId, setActiveToolSectionId] = useState<string>(
    TOOL_SECTIONS[0]?.id ?? "",
  );

  const templates = Array.isArray(apiTemplates) ? apiTemplates : [];

  const isKorean =
    resolveLocale(i18n.resolvedLanguage || i18n.language) === "ko-KR";
  const resolveTemplateDescription = (template: StackTemplate) => {
    const localized = TEMPLATE_DESCRIPTION_I18N[template.id];
    const rawDescription = (template.description ?? "").trim();
    const override = TEMPLATE_DESCRIPTION_LOCALE_OVERRIDES[rawDescription];

    if (override) {
      return isKorean ? override.ko : override.en;
    }

    if (!localized) return template.description;

    if (
      rawDescription.length > 0 &&
      rawDescription !== localized.en &&
      rawDescription !== localized.ko
    ) {
      return rawDescription;
    }

    return isKorean ? localized.ko : localized.en;
  };

  const filtered = templates.filter((template) => {
    const localizedDescription =
      resolveTemplateDescription(template).toLowerCase();
    const normalizedSearch = search.toLowerCase();
    return (
      template.name.toLowerCase().includes(normalizedSearch) ||
      localizedDescription.includes(normalizedSearch) ||
      template.tools.some((tool) =>
        tool.toLowerCase().includes(normalizedSearch),
      )
    );
  });

  const selectedTemplate = selectedTemplateId
    ? (templates.find((template) => template.id === selectedTemplateId) ?? null)
    : null;

  const selectedDetail = selectedTemplate
    ? {
        fullDescription: resolveTemplateDescription(selectedTemplate),
        resource:
          selectedTemplate.minResources ??
          t("stackTemplatePage.modal.na", "N/A"),
        compatibility:
          selectedTemplate.recommendedUseCase ??
          t(
            "stackTemplatePage.modal.compatibilityFallback",
            "Compatibility details are managed in Stack Version.",
          ),
      }
    : null;

  const handleUseTemplate = (template: StackTemplate) => {
    const overrides = buildInstallOverridesFromTemplate(template);
    setTemplate(template.id);
    loadFromTemplate(template.id, overrides);
    navigate(`/stack/install?template=${template.id}`);
  };

  const nextDuplicateTemplateId = (templateId: string) => {
    const existing = new Set(templates.map((item) => item.id));
    const base = `${toTemplateSlug(templateId) || "template"}-copy`;

    if (!existing.has(base)) {
      return base;
    }

    let index = 2;
    while (existing.has(`${base}-${index}`)) {
      index += 1;
    }

    return `${base}-${index}`;
  };

  const handleDuplicateTemplate = (template: StackTemplate) => {
    const duplicateName = `${template.name} ${t("stackTemplatePage.duplicate.suffix", "Duplicate")}`;
    const duplicateId = nextDuplicateTemplateId(template.id);
    const duplicateMinutes = estimateInstallMinutesForTemplate(template);
    const duplicateNs = duplicateMinutes * NS_PER_MINUTE;
    const duplicatedTools =
      template.toolDetails && template.toolDetails.length > 0
        ? template.toolDetails.map((tool) => ({
            category: tool.category,
            name: tool.name,
            helm_version: tool.helm_version,
            app_version: tool.app_version,
          }))
        : (Array.isArray(template.tools) ? template.tools : []).map(
            (toolName) => toToolEntry(toolName),
          );

    createTemplate.mutate(
      {
        id: duplicateId,
        name: duplicateName,
        description: resolveTemplateDescription(template),
        tools: duplicatedTools,
        estimated_install_time: duplicateNs,
        recommended_use_case: template.recommendedUseCase ?? "",
        min_resources: template.minResources ?? "",
        planning_profile: template.planningProfile ?? DEFAULT_PLANNING_PROFILE,
      },
      {
        onError: () => {
          setFormError(
            t(
              "stackTemplatePage.errors.duplicateFailed",
              "Failed to duplicate template.",
            ),
          );
        },
      },
    );
  };

  const resetForm = () => {
    setForm(EMPTY_TEMPLATE_FORM);
    setFormError(null);
    setEditingTemplateId(null);
    setAddToolDrafts(buildInitialAddToolDrafts);
    setActiveToolSectionId(TOOL_SECTIONS[0]?.id ?? "");
    setRemovedBaseSectionIds([]);
  };

  const openCreateModal = () => {
    resetForm();
    setForm((prev) => ({ ...prev, id: createTemplateUUID() }));
    setFormOpen(true);
  };

  const openEditModal = (template: StackTemplate) => {
    const toolsFromDetail = (template.toolDetails ?? [])
      .filter((tool) => tool.name && tool.name.trim().length > 0)
      .map((tool) => toToolEntry(tool.name, tool));

    const tools =
      toolsFromDetail.length > 0
        ? toolsFromDetail
        : (Array.isArray(template.tools) ? template.tools : []).map(
            (toolName) => toToolEntry(toolName),
          );

    const toolCategoryIds = new Set(tools.map((tool) => tool.category));
    // 편집은 템플릿이 실제로 쓰는 섹션만 편다. 다만 도구가 하나도 없는
    // 템플릿(Empty Template)에 그 규칙을 그대로 적용하면 쓰는 섹션이 없어
    // 편집기가 통째로 사라지고, 만들기 모달과 다른 옛 화면이 남는다 — 그때는
    // 만들기와 같이 전부 편다.
    const removedInTemplate =
      tools.length === 0
        ? []
        : TOOL_SECTIONS.filter(
            (section) =>
              !section.categories.some((category) =>
                toolCategoryIds.has(category.category),
              ),
          ).map((section) => section.id);

    // 편집기의 선택 상태는 draft 가 소유한다. 여기서 템플릿 값을 심어 두지
    // 않으면 초기 선택을 보여 주려고 "적용된 값"이 draft 를 이겨야 하고, 그러면
    // 이미 담긴 카테고리는 다른 도구를 눌러도 선택이 움직이지 않는다.
    setAddToolDrafts(() => {
      const seeded = buildInitialAddToolDrafts();
      for (const tool of tools) {
        const section = TOOL_SECTIONS.find((candidate) =>
          candidate.categories.some((category) => category.category === tool.category),
        );
        if (!section) continue;
        seeded[addToolDraftKey(section.id, tool.category)] = { ...tool };
      }
      return seeded;
    });
    setActiveToolSectionId(
      TOOL_SECTIONS.find((section) => !removedInTemplate.includes(section.id))?.id ??
        TOOL_SECTIONS[0]?.id ??
        "",
    );
    setRemovedBaseSectionIds(removedInTemplate);
    setEditingTemplateId(template.id);
    setFormError(null);
    setForm({
      id: template.id,
      name: template.name,
      description: resolveTemplateDescription(template),
      tools,
      recommendedUseCase: template.recommendedUseCase ?? "",
      minResources: template.minResources ?? "",
      // 폼에 싣지 않으면 이름만 고쳐 저장해도 프로파일이 기본값으로 되돌아간다.
      planningProfile: template.planningProfile ?? DEFAULT_PLANNING_PROFILE,
    });
    setFormOpen(true);
  };

  const closeFormModal = () => {
    setFormOpen(false);
    resetForm();
  };

  const handleFormChange = (key: keyof TemplateFormState, value: string) => {
    setForm((prev) => ({ ...prev, [key]: value }));
  };

  const updateAddToolDraft = (
    draftKey: string,
    patch: Partial<AddToolDraft>,
  ) => {
    setAddToolDrafts((prev) => {
      const current = prev[draftKey];
      if (!current) return prev;
      return {
        ...prev,
        [draftKey]: {
          ...current,
          ...patch,
        },
      };
    });
  };

  const submitAddTool = (draftKey: string) => {
    const draft = addToolDrafts[draftKey];
    if (!draft?.category || !draft.name) {
      return;
    }

    setForm((prev) => {
      const existingIndex = prev.tools.findIndex(
        (tool) => tool.category === draft.category,
      );
      const nextEntry: ToolEntry = {
        category: draft.category,
        name: draft.name,
        helm_version: draft.helm_version,
        app_version: draft.app_version,
      };

      if (existingIndex >= 0) {
        return {
          ...prev,
          tools: prev.tools.map((tool, index) =>
            index === existingIndex ? nextEntry : tool,
          ),
        };
      }

      return {
        ...prev,
        tools: [...prev.tools, nextEntry],
      };
    });
  };

  const removeCategoryTool = (sectionId: string, category: string) => {
    setForm((prev) => ({
      ...prev,
      tools: prev.tools.filter((tool) => tool.category !== category),
    }));

    const categoryMeta = TOOL_CATEGORY_LOOKUP.get(category);
    const defaultName = categoryMeta?.options[0] ?? "";
    const defaults = defaultVersionsForTool(defaultName);
    const draftKey = addToolDraftKey(sectionId, category);
    updateAddToolDraft(draftKey, {
      category,
      name: defaultName,
      ...defaults,
    });
  };

  const addSection = () => {
    if (!newSectionLabel.trim() || !newCategoryLabel.trim()) return;
    const sectionId = newSectionLabel.toLowerCase().replace(/\s+/g, "-");
    const categoryId = `${sectionId}_${newCategoryLabel.toLowerCase().replace(/\s+/g, "_")}`;
    const options = newCategoryOptions
      .split(",")
      .map((o) => o.trim())
      .filter(Boolean);

    const newSection: ToolSectionDefinition = {
      id: sectionId,
      label: newSectionLabel.trim(),
      categories: [
        {
          category: categoryId,
          label: newCategoryLabel.trim(),
          options: options.length > 0 ? options : ["Custom Tool"],
        },
      ],
    };
    setCustomSections((prev) => [...prev, newSection]);
    setActiveToolSectionId(sectionId);
    setAddToolDrafts((prev) => {
      const defaultName = options[0] ?? "Custom Tool";
      const defaults = defaultVersionsForTool(defaultName);
      return {
        ...prev,
        [addToolDraftKey(sectionId, categoryId)]: {
          category: categoryId,
          name: defaultName,
          ...defaults,
        },
      };
    });
    setNewSectionLabel("");
    setNewCategoryLabel("");
    setNewCategoryOptions("");
    setAddSectionOpen(false);
  };

  const removeSection = (sectionId: string) => {
    const section =
      allSections.find((s) => s.id === sectionId) ??
      TOOL_SECTIONS.find((s) => s.id === sectionId);
    if (!section) return;
    const categoryIds = new Set(section.categories.map((c) => c.category));
    setForm((prev) => ({
      ...prev,
      tools: prev.tools.filter((t) => !categoryIds.has(t.category)),
    }));

    if (TOOL_SECTIONS.some((base) => base.id === sectionId)) {
      setRemovedBaseSectionIds((prev) =>
        prev.includes(sectionId) ? prev : [...prev, sectionId],
      );
    } else {
      setCustomSections((prev) => prev.filter((s) => s.id !== sectionId));
      setAddToolDrafts((prev) => {
        const next = { ...prev };
        Object.keys(next).forEach((key) => {
          if (key.startsWith(`${sectionId}:`)) delete next[key];
        });
        return next;
      });
    }
  };

  const submitTemplate = () => {
    const templateId = editingTemplateId ?? (form.id || createTemplateUUID());

    const payload = {
      id: templateId,
      name: form.name,
      description: form.description,
      tools: form.tools,
      estimated_install_time: estimatedInstallTimeNs,
      recommended_use_case: form.recommendedUseCase,
      min_resources: form.minResources,
      planning_profile: form.planningProfile,
    };

    if (editingTemplateId) {
      updateTemplate.mutate(
        { ...payload, id: editingTemplateId },
        {
          onSuccess: () => {
            closeFormModal();
          },
          onError: () => {
            setFormError(
              t(
                "stackTemplatePage.errors.updateFailed",
                "Failed to update template.",
              ),
            );
          },
        },
      );
      return;
    }

    createTemplate.mutate(payload, {
      onSuccess: () => {
        closeFormModal();
      },
      onError: () => {
        setFormError(
          t(
            "stackTemplatePage.errors.createFailed",
            "Failed to create template.",
          ),
        );
      },
    });
  };

  const handleDeleteTemplate = () => {
    if (!deleteTemplateId) return;
    deleteTemplate.mutate(deleteTemplateId, {
      onSuccess: () => {
        setDeleteTemplateId(null);
      },
    });
  };

  return (
    <div>
      <PageHeader
        breadcrumb={
          [
            {
              label: t("stackTemplatePage.breadcrumb.stackList", "Stack List"),
              path: "/stack/list",
            },
            {
              label: t("stackTemplatePage.breadcrumb.current", "Stack Template"),
            },
          ]
        }
        icon={<BookOpen {...iconProps('sm')} />}
        tone="success"
        title={t("stackTemplatePage.title", "Stack Template")}
        subtitle={
          t(
            "stackTemplatePage.description",
            "Select a validated DevSecOps stack template to get started quickly.",
          )
        }
        actions={
          isAdmin && (
            <Button
              variant="primary"
              size="md"
              type="button"
              onClick={openCreateModal}
            >
              <Plus {...iconProps('sm')} />
              {t("stackTemplatePage.actions.createTemplate", "Create Template")}
            </Button>
          )
        }
      />

      {/* Search */}
      <div className="mb-5 max-w-[360px]">
        <SearchInput
          wrapperClassName="w-[220px]"
          placeholder={t(
          "stackTemplatePage.searchPlaceholder",
          "Search templates...",
          )}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
      </div>

      {/* Template cards */}
      <div className="grid grid-cols-[repeat(auto-fill,minmax(340px,1fr))] gap-4">
        {filtered.map((template) => (
          <div
            key={template.id}
            data-tour="template-card"
            data-tour-template={template.id}
            className="flex h-full flex-col gap-[14px] rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-[var(--card-padding)] transition-colors duration-150 hover:border-[var(--color-border-hover)]"
          >
            {/* Card header */}
            <div>
              <h3 className="m-0 mb-1 text-[15px] font-bold text-[var(--color-text-primary)]">
                {template.name}
              </h3>
              <p className="m-0 text-[13px] leading-[1.5] text-[var(--color-text-secondary)]">
                {resolveTemplateDescription(template)}
              </p>
            </div>

            {/* Info grid */}
            <div className="grid grid-cols-2 gap-3 rounded-lg bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)] p-3">
              <div>
                <span className="text-[11px] font-semibold text-[var(--color-text-muted)]">
                  {t("stackTemplatePage.card.estimatedTime", "설치 시간")}
                </span>
                <p className="m-0 mt-1 text-[13px] font-semibold text-[var(--color-text-primary)]">
                  {t('stackTemplatePage.card.minutes', '{{minutes}} min', { minutes: estimateInstallMinutesForTemplate(template) })}
                </p>
              </div>
              {template.recommendedUseCase && (
                <div>
                  <span className="text-[11px] font-semibold text-[var(--color-text-muted)]">
                    {t("stackTemplatePage.card.recommendedUse", "권장 사용")}
                  </span>
                  <p className="m-0 mt-1 text-[13px] font-semibold text-[var(--color-text-primary)]">
                    {template.recommendedUseCase}
                  </p>
                </div>
              )}
              {template.minResources && (
                <div className="col-span-2">
                  <span className="text-[11px] font-semibold text-[var(--color-text-muted)]">
                    {t("stackTemplatePage.card.minResources", "최소 리소스")}
                  </span>
                  <p className="m-0 mt-1 text-[12px] text-[var(--color-text-primary)]">
                    {template.minResources}
                  </p>
                </div>
              )}
            </div>

            {/* Tools preview */}
            <div>
              <span className="text-[11px] font-semibold text-[var(--color-text-muted)]">
                {t("stackTemplatePage.card.includedTools", "포함된 도구")}
              </span>
              <div className="mt-2 flex flex-wrap gap-1.5">
                {uniqueTools(template.tools).slice(0, 4).map((tool) => (
                  <span
                    key={tool}
                    className="rounded-md bg-[color-mix(in_srgb,_var(--color-primary)_12%,_transparent)] px-2 py-1 text-[11px] font-semibold text-[var(--color-primary)]"
                  >
                    {tool}
                  </span>
                ))}
                {uniqueTools(template.tools).length > 4 && (
                  <span className="rounded-md bg-[color-mix(in_srgb,_var(--color-text-muted)_12%,_transparent)] px-2 py-1 text-[11px] font-semibold text-[var(--color-text-muted)]">
                    +{uniqueTools(template.tools).length - 4}
                  </span>
                )}
              </div>
            </div>

            {/* Footer */}
            <div className="mt-auto border-t border-[var(--color-border-default)] pt-3">
              <div className="flex items-center gap-1.5">
                {isAdmin && (
                  <>
                    <Button
                      variant="ghost"
                      size="sm"
                      type="button"
                      onClick={() => openEditModal(template)}
                    >
                      <Pencil {...iconProps('xs')} />
                      {t("stackTemplatePage.actions.edit", "Edit")}
                    </Button>
                    <Button
                      variant="danger"
                      size="sm"
                      type="button"
                      onClick={() => setDeleteTemplateId(template.id)}
                    >
                      <Trash2 {...iconProps('xs')} />
                      {t("stackTemplatePage.actions.delete", "Delete")}
                    </Button>
                  </>
                )}
                <Button
                  variant="outline"
                  size="sm"
                  type="button"
                  className="ml-auto"
                  onClick={() => setSelectedTemplateId(template.id)}
                  data-tour="template-detail"
                >
                  <ExternalLink {...iconProps('xs')} />
                  {t("stackTemplatePage.actions.viewDetail", "상세 보기")}
                </Button>
              </div>
            </div>
          </div>
        ))}
      </div>

      {filtered.length === 0 && (
        <div className="py-[60px] text-center text-sm text-[var(--color-text-secondary)]">
          {t("stackTemplatePage.empty", "No search results found.")}
        </div>
      )}

      <Modal
        open={selectedTemplate !== null}
        onClose={() => setSelectedTemplateId(null)}
        title={
          selectedTemplate?.name ??
          t("stackTemplatePage.modal.templateDetail", "Template Detail")
        }
        wide
        footer={
          <>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setSelectedTemplateId(null)}
              type="button"
            >
              {t("stackTemplatePage.actions.close", "Close")}
            </Button>
            {selectedTemplate && (
              <>
                {isAdmin && (
                  <>
                    <Button
                      variant="ghost"
                      size="sm"
                      type="button"
                      onClick={() => {
                        setSelectedTemplateId(null);
                        openEditModal(selectedTemplate);
                      }}
                    >
                      <Pencil {...iconProps('xs')} />
                      {t("stackTemplatePage.actions.edit", "Edit")}
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      type="button"
                      onClick={() => {
                        setSelectedTemplateId(null);
                        handleDuplicateTemplate(selectedTemplate);
                      }}
                    >
                      {t(
                        "stackTemplatePage.actions.duplicateTemplate",
                        "Duplicate Template",
                      )}
                    </Button>
                  </>
                )}
                <Button
                  variant="primary"
                  size="sm"
                  type="button"
                  className="bg-[linear-gradient(135deg,var(--color-success),var(--color-success))] text-white"
                  onClick={() => {
                    setSelectedTemplateId(null);
                    handleUseTemplate(selectedTemplate);
                  }}
                  data-tour="use-base-template"
                >
                  {t(
                    "stackTemplatePage.actions.useBaseTemplate",
                    "Use Base Template",
                  )}
                </Button>
              </>
            )}
          </>
        }
      >
        {selectedTemplate && selectedDetail && (
          <div className="flex flex-col gap-4">
            <div>
              <h4 className="mb-1 text-sm font-semibold text-[var(--color-text-primary)]">
                {t("stackTemplatePage.modal.description", "Description")}
              </h4>
              <p className="m-0 text-sm text-[var(--color-text-secondary)]">
                {selectedDetail.fullDescription}
              </p>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div>
                <span className="text-xs font-semibold text-[var(--color-text-muted)]">
                  {t(
                    "stackTemplatePage.modal.estimatedDeployTime",
                    "설치 시간",
                  )}
                </span>
                <p className="m-0 mt-1 text-sm font-semibold text-[var(--color-text-primary)]">
                  {t('stackTemplatePage.card.minutes', '{{minutes}} min', { minutes: estimateInstallMinutesForTemplate(selectedTemplate) })}
                </p>
              </div>
              {selectedDetail.compatibility && (
                <div>
                  <span className="text-xs font-semibold text-[var(--color-text-muted)]">
                    {t("stackTemplatePage.modal.compatibility", "권장 사용")}
                  </span>
                  <p className="m-0 mt-1 text-sm font-semibold text-[var(--color-text-primary)]">
                    {selectedDetail.compatibility}
                  </p>
                </div>
              )}
              <div>
                <span className="text-xs font-semibold text-[var(--color-text-muted)]">
                  {t("stackTemplatePage.form.planningProfile", "Sizing Profile")}
                </span>
                {/* 최소 리소스 옆에 둔다. "4 vCPU / 8Gi" 가 어느 규모로 계획했을
                    때의 값인지는 이 둘을 함께 봐야 읽힌다. */}
                <p className="m-0 mt-1 text-sm font-semibold text-[var(--color-text-primary)]">
                  {t(
                    `stackTemplatePage.form.planningProfileOption.${selectedTemplate.planningProfile}`,
                    selectedTemplate.planningProfile,
                  )}
                </p>
              </div>
              {selectedDetail.resource && (
                <div className="col-span-2">
                  <span className="text-xs font-semibold text-[var(--color-text-muted)]">
                    {t(
                      "stackTemplatePage.modal.resourceRequirements",
                      "최소 리소스",
                    )}
                  </span>
                  <p className="m-0 mt-1 text-sm text-[var(--color-text-primary)]">
                    {selectedDetail.resource}
                  </p>
                </div>
              )}
            </div>

            <div>
              <h4 className="mb-2 text-sm font-semibold text-[var(--color-text-primary)]">
                {t("stackTemplatePage.modal.includedTools", "Included Tools")}
              </h4>
              <p className="m-0 mb-2 text-xs text-[var(--color-text-muted)]">
                {t(
                  "stackTemplatePage.modal.matrixVersionNote",
                  "These versions reflect the validated matrix snapshot used for template guidance.",
                )}
              </p>
              {selectedTemplate.toolDetails &&
              selectedTemplate.toolDetails.length > 0 ? (
                <div className="space-y-2">
                  {selectedTemplate.toolDetails.map((tool) => {
                    const color = getToolCategoryColor(tool.category);
                    return (
                      <div
                        key={`${tool.category}-${tool.name}`}
                        className="flex items-center justify-between rounded-lg border border-[var(--color-border-default)] p-2.5"
                      >
                        <div className="flex items-center gap-2">
                          <span
                            className="rounded-md px-2 py-1 text-[11px] font-semibold"
                            style={{
                              backgroundColor: color.bg,
                              color: color.color,
                            }}
                          >
                            {tool.category}
                          </span>
                          <span className="text-sm font-semibold text-[var(--color-text-primary)]">
                            {tool.name}
                          </span>
                        </div>
                        <div className="flex items-center gap-2 text-xs text-[var(--color-text-muted)]">
                          <span>Matrix Helm: {tool.helm_version}</span>
                          <span>Matrix App: {tool.app_version}</span>
                        </div>
                      </div>
                    );
                  })}
                </div>
              ) : (
                <div className="grid grid-cols-[repeat(auto-fill,minmax(180px,1fr))] gap-2">
                  {uniqueTools(selectedTemplate.tools).map((tool) => (
                    <div
                      key={tool}
                      className="flex items-center gap-2 rounded-lg border border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)] px-2.5 py-2"
                    >
                      <Wrench {...iconProps('xs')} color="var(--color-warning)" />
                      <span className="text-[13px] font-semibold text-[var(--color-text-primary)]">
                        {tool}
                      </span>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        )}
      </Modal>

      <Modal
        open={formOpen}
        onClose={closeFormModal}
        title={
          editingTemplateId
            ? t("stackTemplatePage.modal.editTitle", "Edit Template")
            : t("stackTemplatePage.modal.createTitle", "Create Template")
        }
        size="xl"
        footer={
          <>
            <Button
              variant="outline"
              size="sm"
              onClick={closeFormModal}
              type="button"
            >
              {t("common.cancel", "Cancel")}
            </Button>
            <Button
              variant="primary"
              size="sm"
              type="button"
              onClick={submitTemplate}
              loading={createTemplate.isPending || updateTemplate.isPending}
            >
              {editingTemplateId
                ? t("common.save", "Save")
                : t("stackTemplatePage.actions.create", "Create")}
            </Button>
          </>
        }
      >
        <div className="flex flex-col gap-3">
          <div className="grid gap-3 md:grid-cols-2">
            <Input
              label={t("stackTemplatePage.form.templateId", "Template ID")}
              value={editingTemplateId ?? form.id}
              onChange={(event) => handleFormChange("id", event.target.value)}
              disabled={editingTemplateId !== null}
            />
            <Input
              label={t("stackTemplatePage.form.name", "Name")}
              value={form.name}
              onChange={(event) => handleFormChange("name", event.target.value)}
            />
          </div>
          <Input
            label={t("stackTemplatePage.form.description", "Description")}
            value={form.description}
            onChange={(event) =>
              handleFormChange("description", event.target.value)
            }
          />
          <div className="flex flex-col gap-2">
            <div className="text-xs font-medium tracking-[0.02em] text-[var(--color-text-secondary)]">
              {t("stackTemplatePage.form.tools", "Tools")}
            </div>
            <div className="text-[11px] text-[var(--color-text-muted)]">
              {t(
                "stackTemplatePage.form.toolsHint",
                "Create the template scaffold first, then add or refine tools by section.",
              )}
            </div>
            <TemplateToolSections
              sections={allSections}
              activeSectionId={activeToolSectionId}
              onSelectSection={setActiveToolSectionId}
              getCategories={getSectionCategories}
              tools={form.tools}
              drafts={addToolDrafts}
              draftKeyFor={addToolDraftKey}
              defaultVersionsFor={defaultVersionsForTool}
              onDraftChange={updateAddToolDraft}
              onSubmitTool={submitAddTool}
              onRemoveTool={removeCategoryTool}
              onRemoveSection={removeSection}
            />

            {addSectionOpen ? (
              <div className="mt-2 flex flex-col gap-2 rounded-lg border border-dashed border-[var(--color-border-default)] p-3">
                <TextInput
                  type="text"
                  value={newSectionLabel}
                  onChange={(e) => setNewSectionLabel(e.target.value)}
                  placeholder={t(
                    "stackTemplatePage.form.sectionNamePlaceholder",
                    "Section name (e.g. Security)",
                  )}
                />
                <TextInput
                  type="text"
                  value={newCategoryLabel}
                  onChange={(e) => setNewCategoryLabel(e.target.value)}
                  placeholder={t(
                    "stackTemplatePage.form.firstCategoryPlaceholder",
                    "First category (e.g. Scanner)",
                  )}
                />
                <TextInput
                  type="text"
                  value={newCategoryOptions}
                  onChange={(e) => setNewCategoryOptions(e.target.value)}
                  placeholder={t(
                    "stackTemplatePage.form.toolOptionsPlaceholder",
                    "Tool options (comma separated, e.g. Trivy, SonarQube)",
                  )}
                />
                <div className="flex gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    type="button"
                    onClick={addSection}
                  >
                    {t("stackTemplatePage.actions.addSection", "Add Section")}
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    type="button"
                    onClick={() => setAddSectionOpen(false)}
                  >
                    {t("common.cancel", "Cancel")}
                  </Button>
                </div>
              </div>
            ) : (
              <button
                type="button"
                onClick={() => setAddSectionOpen(true)}
                className="mt-2 flex w-full cursor-pointer items-center justify-center gap-1.5 rounded-lg border border-dashed border-[var(--color-border-default)] px-3 py-2.5 text-sm text-[var(--color-text-secondary)] transition-colors hover:border-[var(--color-border-hover)] hover:text-[var(--color-text-primary)]"
              >
                <Plus {...iconProps('sm')} />
                {t("stackTemplatePage.actions.addSection", "Add Section")}
              </button>
            )}
          </div>
          <div className="rounded-lg border border-[var(--color-border-default)] bg-[color-mix(in_srgb,_var(--color-text-primary)_2%,_transparent)] px-3 py-2.5">
            <div className="text-xs font-medium tracking-[0.02em] text-[var(--color-text-secondary)]">
              {t(
                "stackTemplatePage.form.estimatedInstallTimeAuto",
                "Estimated Install Time (Auto)",
              )}
            </div>
            <div className="mt-1 text-sm font-semibold text-[var(--color-text-primary)]">
              {t(
                "stackTemplatePage.form.estimatedInstallTimeValue",
                "{{minutes}} min ({{nanoseconds}} ns)",
                {
                  minutes: estimatedInstallMinutes,
                  nanoseconds: estimatedInstallTimeNs,
                },
              )}
            </div>
          </div>
          <div className="grid gap-3 md:grid-cols-2">
            <Input
              label={t(
                "stackTemplatePage.form.recommendedUseCase",
                "Recommended Use Case",
              )}
              value={form.recommendedUseCase}
              onChange={(event) =>
                handleFormChange("recommendedUseCase", event.target.value)
              }
            />
            <Input
              label={t(
                "stackTemplatePage.form.minimumResources",
                "Minimum Resources",
              )}
              value={form.minResources}
              onChange={(event) =>
                handleFormChange("minResources", event.target.value)
              }
            />
          </div>
          <div className="flex flex-col gap-1">
            <Select
              label={t("stackTemplatePage.form.planningProfile", "Sizing Profile")}
              value={form.planningProfile}
              onChange={(event) =>
                setForm((prev) => ({
                  ...prev,
                  planningProfile: event.target.value as PlanningProfile,
                }))
              }
            >
              {PLANNING_PROFILE_VALUES.map((profile) => (
                <option key={profile} value={profile}>
                  {t(`stackTemplatePage.form.planningProfileOption.${profile}`, profile)}
                </option>
              ))}
            </Select>
            {/* 고른 도구만으로는 스택이 몇 Gi 를 먹을지 정해지지 않는다.
                여기서 고른 규모가 설치 마법사의 리소스 계획 시작값이 된다. */}
            <span className="text-[11px] text-[var(--color-text-muted)]">
              {t(
                "stackTemplatePage.form.planningProfileHint",
                "The install wizard starts its resource plan from this size.",
              )}
            </span>
          </div>
          {formError && (
            <div className="text-xs text-[var(--color-error)]">{formError}</div>
          )}
        </div>
      </Modal>

      <ConfirmDialog
        open={deleteTemplateId !== null}
        onClose={() => setDeleteTemplateId(null)}
        onConfirm={handleDeleteTemplate}
        title={t("stackTemplatePage.confirm.deleteTitle", "Delete Template")}
        description={t(
          "stackTemplatePage.confirm.deleteDescription",
          "This template will no longer be shown in the list. Continue?",
        )}
        confirmLabel={t(
          "stackTemplatePage.confirm.deleteConfirmLabel",
          "Delete Template",
        )}
        loading={deleteTemplate.isPending}
      />
    </div>
  );
}
