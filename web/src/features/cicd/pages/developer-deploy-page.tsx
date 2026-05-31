import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useFieldArray, useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useNavigate, useSearchParams } from "react-router-dom";
import { ChevronRight, Plus, RefreshCw, Rocket, Trash2 } from "lucide-react";
import { Breadcrumb } from "../../../components/shared/breadcrumb";
import { CodePreview } from "../../../components/shared/code-preview";
import { Button } from "../../../components/ui/button";
import { Input } from "../../../components/ui/input";
import { NativeSelect } from "../../../components/ui/native-select";
import { cn } from "../../../lib/utils";
import { useClusterNamespaces, useClusters } from "../../admin/api/admin-api";
import { useStackIntegrations, useStacks } from "../../stack/api/stack-api";
import {
  useCicdTemplates,
  useCreatePipeline,
  useDeployPipeline,
} from "../api/cicd-api";
import type { AppType } from "../api/cicd-api";
import { StepSection, labelStyleClass } from "../components/deploy-ui";
import { generateManifestYamls } from "../utils/yaml-generator";

type Step = 1 | 2 | 3 | 4 | 5 | 6;
type Capability = "CI" | "CD" | "Test" | "Security";
type PipelinePhase = "production" | "qa" | "development";

const CAPABILITIES: Capability[] = ["CI", "CD", "Test", "Security"];
const DISABLED_CAPABILITIES = new Set<Capability>(["Test", "Security"]);
const PHASES: PipelinePhase[] = ["production", "qa", "development"];

const STEP_LABEL_DEFAULTS: Record<Step, string> = {
  1: "Basic Info",
  2: "Code Checkout",
  3: "Build",
  4: "Test",
  5: "Security",
  6: "Create",
};

interface EnvVar {
  key: string;
  value: string;
}

interface FormState {
  appName: string;
  gitUrl: string;
  serviceUrl: string;
  configRepositoryUrl: string;
  dockerfileBranch: string;
  deployYamlBranch: string;
  manifestPath: string;
  dockerfilePath: string;
  dockerContext: string;
  clusterId: string;
  namespace: string;
  replicas: number;
  cpuRequest: string;
  cpuLimit: string;
  memoryRequest: string;
  memoryLimit: string;
  envVars: EnvVar[];
}

const DEFAULT_REPOSITORY_PATH = "root/spring-sample.git";
const DEFAULT_DOCKERFILE_REPOSITORY_PATH =
  "root/spring-sample/-/blob/master/Dockerfile";
const DEFAULT_IMAGE_REGISTRY_PATH = "root/spring-sample";
const DEFAULT_DEPLOY_YAML_PATH = "root/spring-sample/deploy";
const MANIFEST_ENV_KEYS = {
  deployment: "NULLUS_MANIFEST_DEPLOYMENT",
  service: "NULLUS_MANIFEST_SERVICE",
  ingress: "NULLUS_MANIFEST_INGRESS",
} as const;

const deploySchema = z.object({
  appName: z
    .string()
    .min(2, "App name must be at least 2 characters")
    .max(50, "App name must be 50 characters or less"),
  gitUrl: z.string(),
  serviceUrl: z.string(),
  configRepositoryUrl: z.string(),
  dockerfileBranch: z.string(),
  deployYamlBranch: z.string(),
  manifestPath: z.string(),
  dockerfilePath: z.string(),
  dockerContext: z.string(),
  clusterId: z.string().min(1, "Cluster is required"),
  namespace: z.string().min(1, "Namespace is required"),
  replicas: z.number().min(1).max(10),
  cpuRequest: z.string().min(1, "CPU request is required"),
  cpuLimit: z.string().min(1, "CPU limit is required"),
  memoryRequest: z.string().min(1, "Memory request is required"),
  memoryLimit: z.string().min(1, "Memory limit is required"),
  envVars: z
    .array(z.object({ key: z.string(), value: z.string() }))
    .superRefine((envVars, ctx) => {
      envVars.forEach((env, index) => {
        if (env.value.trim() && !env.key.trim()) {
          ctx.addIssue({
            code: "custom",
            message: "Key is required when value exists",
            path: [index, "key"],
          });
        }
      });
    }),
});

const DEFAULT_FORM: FormState = {
  appName: "",
  gitUrl: "",
  serviceUrl: "",
  configRepositoryUrl: "",
  dockerfileBranch: "main",
  deployYamlBranch: "main",
  manifestPath: DEFAULT_DEPLOY_YAML_PATH,
  dockerfilePath: "",
  dockerContext: "",
  clusterId: "",
  namespace: "default",
  replicas: 2,
  cpuRequest: "100m",
  cpuLimit: "500m",
  memoryRequest: "128Mi",
  memoryLimit: "512Mi",
  envVars: [{ key: "", value: "" }],
};

const sectionClassName =
  "rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] p-6";

function RequiredDot() {
  return (
    <span
      aria-hidden="true"
      data-testid="required-dot"
      className="ml-1 inline-block h-1.5 w-1.5 rounded-full bg-[#ef4444] align-middle"
    />
  );
}

function createPipelineErrorMessage(error: unknown) {
  const rawMessage =
    typeof error === "object" &&
    error !== null &&
    "message" in error &&
    typeof error.message === "string"
      ? error.message
      : error instanceof Error
        ? error.message
        : "";
  const normalized = rawMessage.toLowerCase();

  if (
    normalized.includes("different organization") ||
    normalized.includes("org mismatch")
  ) {
    return "선택한 Stack에 접근할 수 없습니다. 현재 조직의 Stack을 다시 선택하거나 관리자에게 권한을 요청하세요.";
  }
  if (normalized.includes("stack") && normalized.includes("not found")) {
    return "선택한 Stack을 찾을 수 없습니다. Stack 목록을 새로고침한 뒤 다시 선택하세요.";
  }
  if (normalized.includes("template") && normalized.includes("not found")) {
    return "선택한 템플릿을 찾을 수 없습니다. 템플릿을 다시 선택한 뒤 생성하세요.";
  }
  if (normalized.includes("cluster")) {
    return "배포할 Cluster 정보를 확인할 수 없습니다. Cluster를 다시 선택한 뒤 생성하세요.";
  }
  if (rawMessage) {
    return `Pipeline을 생성하지 못했습니다. 관리자에게 아래 오류를 전달하세요: ${rawMessage}`;
  }
  return "Failed to create pipeline.";
}

export function DeveloperDeployPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [activeStep, setActiveStep] = useState<Step>(1);
  const [selectedStackId, setSelectedStackId] = useState("");
  const [repoName, setRepoName] = useState(DEFAULT_REPOSITORY_PATH);
  const [selectedTemplateId, setSelectedTemplateId] = useState("");
  const [selectedAppType, setSelectedAppType] = useState<AppType>(
    (searchParams.get("appType") ?? "backend") as AppType,
  );
  const [selectedCapabilities, setSelectedCapabilities] = useState<
    Capability[]
  >(["CI", "CD"]);
  const [selectedPhase, setSelectedPhase] =
    useState<PipelinePhase>("production");
  const [buildRepositoryPath, setBuildRepositoryPath] = useState(
    DEFAULT_DOCKERFILE_REPOSITORY_PATH,
  );
  const [imageRegistryPath, setImageRegistryPath] = useState(
    DEFAULT_IMAGE_REGISTRY_PATH,
  );
  const [loadedManifests, setLoadedManifests] = useState<
    Partial<ReturnType<typeof generateManifestYamls>>
  >({});
  const [manifestLoadError, setManifestLoadError] = useState("");
  const [isLoadingManifests, setIsLoadingManifests] = useState(false);
  const [createNewNamespace, setCreateNewNamespace] = useState(false);
  const [submitError, setSubmitError] = useState("");
  const sectionRefs = useRef<Record<Step, HTMLDivElement | null>>({
    1: null,
    2: null,
    3: null,
    4: null,
    5: null,
    6: null,
  });

  const { data: stacksData } = useStacks();
  const stacks = (stacksData?.items ?? []).map((stack) => ({
    id: stack.id,
    name: stack.name,
  }));
  const { data: clustersData } = useClusters();
  const { data: templatesData } = useCicdTemplates();
  const templates = useMemo(() => templatesData ?? [], [templatesData]);
  const clusters = useMemo(
    () =>
      (clustersData?.items ?? [])
        .filter((cluster) => {
          const rawTypes =
            Array.isArray(cluster.types) && cluster.types.length > 0
              ? cluster.types
              : cluster.type
                ? [cluster.type]
                : [];
          return rawTypes
            .flatMap((type) => type.split(","))
            .map((type) => type.trim().toLowerCase())
            .includes("target");
        })
        .map((cluster) => ({ id: cluster.id, name: cluster.name })),
    [clustersData],
  );

  const {
    register,
    control,
    watch,
    setValue,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<FormState>({
    resolver: zodResolver(deploySchema),
    defaultValues: DEFAULT_FORM,
    mode: "onChange",
  });
  const { fields, append, remove } = useFieldArray({
    control,
    name: "envVars",
  });
  const form = watch();
  const { data: namespacesData } = useClusterNamespaces(form.clusterId);
  const { data: integrationsData } = useStackIntegrations(selectedStackId);
  const namespaces = useMemo(
    () => (namespacesData ?? []).map((namespace) => namespace.name),
    [namespacesData],
  );
  const namespaceOptions = useMemo(
    () =>
      Array.from(
        new Set([
          "default",
          ...namespaces.filter(
            (namespace) => namespace && namespace !== "default",
          ),
        ]),
      ),
    [namespaces],
  );
  const codeRepositoryEndpoint =
    integrationsData?.integrations.find(
      (integration) => integration.component_type === "code_repository",
    )?.endpoint ?? "";
  const artifactRepositoryEndpoint =
    integrationsData?.integrations.find(
      (integration) => integration.component_type === "package_registry",
    )?.endpoint ?? "";
  const imageRegistryEndpoint =
    integrationsData?.integrations.find(
      (integration) => integration.component_type === "image_registry",
    )?.endpoint ?? "";
  const stackGitBaseUrl =
    selectedStackId && codeRepositoryEndpoint
      ? `${codeRepositoryEndpoint.replace(/\/+$/, "")}/`
      : "";
  const artifactRepositoryBaseUrl =
    selectedStackId && artifactRepositoryEndpoint
      ? `${artifactRepositoryEndpoint.replace(/\/+$/, "")}/`
      : "";
  const imageRegistryBaseUrl =
    selectedStackId && imageRegistryEndpoint
      ? `${imageRegistryEndpoint.replace(/\/+$/, "")}/`
      : "";
  const gitUrl =
    selectedStackId && stackGitBaseUrl
      ? repoName.trim()
        ? `${stackGitBaseUrl}${repoName}`
        : ""
      : form.gitUrl;
  const dockerfileUrl =
    selectedStackId && stackGitBaseUrl && buildRepositoryPath.trim()
      ? `${stackGitBaseUrl}${buildRepositoryPath.trim()}`
      : "";
  const imageRegistryUrl =
    selectedStackId && imageRegistryBaseUrl && imageRegistryPath.trim()
      ? `${imageRegistryBaseUrl}${imageRegistryPath.trim()}`
      : imageRegistryPath.trim();
  const repositoryUrl =
    selectedStackId && stackGitBaseUrl ? gitUrl : form.gitUrl;

  const setField = (
    key: keyof FormState,
    value: FormState[keyof FormState],
  ) => {
    setValue(key as never, value as never, {
      shouldValidate: true,
      shouldDirty: true,
    });
  };

  const firstClusterId = clusters[0]?.id ?? "";
  const firstStackId = stacks[0]?.id ?? "";
  useEffect(() => {
    if (firstClusterId && !form.clusterId) {
      setValue("clusterId", firstClusterId, { shouldValidate: true });
    }
  }, [firstClusterId, form.clusterId, setValue]);

  useEffect(() => {
    if (firstStackId && !selectedStackId) {
      setSelectedStackId(firstStackId);
    }
  }, [firstStackId, selectedStackId]);

  useEffect(() => {
    if (selectedStackId && stackGitBaseUrl) {
      setValue("gitUrl", gitUrl, { shouldValidate: true, shouldDirty: true });
      setValue("configRepositoryUrl", gitUrl, {
        shouldValidate: true,
        shouldDirty: true,
      });
    }
  }, [gitUrl, selectedStackId, setValue, stackGitBaseUrl]);

  const clusterIdParam = searchParams.get("clusterId") ?? "";
  const namespaceParam = searchParams.get("namespace") ?? "";
  const appNameParam = searchParams.get("appName") ?? "";
  useEffect(() => {
    if (clusterIdParam) {
      setValue("clusterId", clusterIdParam, { shouldValidate: true });
    }
    if (namespaceParam) {
      setValue("namespace", namespaceParam, { shouldValidate: true });
      setCreateNewNamespace(!namespaceOptions.includes(namespaceParam));
    }
    if (appNameParam) {
      setValue("appName", appNameParam, { shouldValidate: true });
    }
  }, [
    appNameParam,
    clusterIdParam,
    namespaceOptions,
    namespaceParam,
    setValue,
  ]);

  const templateIdParam = searchParams.get("template") ?? "";
  useEffect(() => {
    if (
      !templateIdParam ||
      templates.length === 0 ||
      selectedTemplateId === templateIdParam
    ) {
      return;
    }
    const template = templates.find((item) => item.id === templateIdParam);
    if (!template) {
      return;
    }

    setSelectedTemplateId(template.id);
    setSelectedAppType(template.appType);
    setRepoName((current) => current || DEFAULT_REPOSITORY_PATH);
    setBuildRepositoryPath(
      (current) => current || DEFAULT_DOCKERFILE_REPOSITORY_PATH,
    );
    setImageRegistryPath((current) => current || DEFAULT_IMAGE_REGISTRY_PATH);
    setValue("manifestPath", DEFAULT_DEPLOY_YAML_PATH, {
      shouldValidate: true,
      shouldDirty: true,
    });
    setValue(
      "appName",
      template.id.replace(/-v\d+$/, "").replace(/^nullus-/, ""),
      { shouldValidate: true, shouldDirty: true },
    );
    setValue("gitUrl", template.gitRepoUrl ?? "", {
      shouldValidate: true,
      shouldDirty: true,
    });
    setValue("dockerfilePath", template.dockerfilePath ?? "", {
      shouldValidate: true,
      shouldDirty: true,
    });
    setValue("dockerContext", template.dockerContext ?? "", {
      shouldValidate: true,
      shouldDirty: true,
    });
    if (template.envVars && Object.keys(template.envVars).length > 0) {
      const envVars = Object.entries(template.envVars).map(([key, value]) => ({
        key,
        value,
      }));
      setValue("envVars", [...envVars, { key: "", value: "" }], {
        shouldValidate: true,
        shouldDirty: true,
      });
    }
  }, [selectedTemplateId, setValue, templateIdParam, templates]);

  const firstNamespace = namespaceOptions[0] ?? "default";
  useEffect(() => {
    if (
      !createNewNamespace &&
      (!form.namespace || !namespaceOptions.includes(form.namespace))
    ) {
      setValue("namespace", firstNamespace, { shouldValidate: true });
    }
  }, [
    createNewNamespace,
    firstNamespace,
    form.namespace,
    namespaceOptions,
    setValue,
  ]);

  const createPipelineMutation = useCreatePipeline();
  const deployPipelineMutation = useDeployPipeline();
  const isSelected = (capability: Capability) =>
    selectedCapabilities.includes(capability);
  const canReview =
    isSelected("CD") &&
    form.appName.trim().length >= 2 &&
    (!isSelected("CI") || gitUrl.trim().length > 0) &&
    !!form.clusterId &&
    !!form.namespace.trim() &&
    form.replicas >= 1 &&
    !!form.cpuRequest.trim() &&
    !!form.cpuLimit.trim() &&
    !!form.memoryRequest.trim() &&
    !!form.memoryLimit.trim() &&
    !errors.appName &&
    (!isSelected("CI") || !errors.gitUrl) &&
    !errors.clusterId &&
    !errors.namespace &&
    !errors.replicas &&
    !errors.cpuRequest &&
    !errors.cpuLimit &&
    !errors.memoryRequest &&
    !errors.memoryLimit &&
    !errors.envVars;
  const generatedManifests = generateManifestYamls(form, selectedAppType);
  const reviewManifests = { ...generatedManifests, ...loadedManifests };
  const toggleCapability = (capability: Capability) => {
    if (DISABLED_CAPABILITIES.has(capability)) {
      return;
    }
    setSelectedCapabilities((current) =>
      current.includes(capability)
        ? current.filter((item) => item !== capability)
        : [...current, capability],
    );
  };
  const scrollToStep = (step: Step) => {
    setActiveStep(step);
    sectionRefs.current[step]?.scrollIntoView({
      behavior: "smooth",
      block: "start",
    });
  };
  const loadDeployYamlDirectory = async () => {
    setManifestLoadError("");
    const directory = form.manifestPath.trim().replace(/\/+$/, "");
    const baseUrl =
      selectedStackId && artifactRepositoryBaseUrl
        ? artifactRepositoryBaseUrl
        : form.configRepositoryUrl.trim();

    if (!baseUrl || !directory) {
      setManifestLoadError(
        "Deploy YAML Repository의 기준 URL과 directory 경로를 확인하세요.",
      );
      return;
    }

    const base = baseUrl.endsWith("/") ? baseUrl : `${baseUrl}/`;
    const loadFile = async (fileName: string) => {
      const response = await fetch(new URL(`${directory}/${fileName}`, base));
      if (!response.ok) {
        throw new Error(`${fileName} 파일을 불러오지 못했습니다.`);
      }
      return response.text();
    };

    setIsLoadingManifests(true);
    try {
      const [deployment, service, ingress] = await Promise.all([
        loadFile("deployment.yaml"),
        loadFile("service.yaml"),
        loadFile("ingress.yaml"),
      ]);
      setLoadedManifests({ deployment, service, ingress });
    } catch {
      setManifestLoadError(
        "해당 directory에서 deployment.yaml, service.yaml, ingress.yaml을 불러오지 못했습니다. 경로와 파일명을 확인하세요.",
      );
    } finally {
      setIsLoadingManifests(false);
    }
  };
  const onSubmit = async (data: FormState) => {
    setSubmitError("");
    try {
      const envVars: Record<string, string> = {};
      data.envVars.forEach(({ key, value }) => {
        if (key.trim()) {
          envVars[key.trim()] = value;
        }
      });

      const hasCi = isSelected("CI");
      const hasCd = isSelected("CD");
      const executionMode = hasCi && hasCd ? "ci_cd" : hasCi ? "ci" : "cd";

      const pipeline = await createPipelineMutation.mutateAsync({
        name: data.appName,
        appType: selectedAppType,
        clusterId: data.clusterId,
        namespace: data.namespace,
        templateId: selectedTemplateId || undefined,
        stackId: selectedStackId || undefined,
        gitRepoUrl: gitUrl,
        dockerfilePath: dockerfileUrl || data.dockerfilePath,
        dockerContext: data.dockerContext,
        executionMode,
        envVars: {
          ...envVars,
          ...(imageRegistryUrl ? { IMAGE_REGISTRY_URL: imageRegistryUrl } : {}),
          [MANIFEST_ENV_KEYS.deployment]: reviewManifests.deployment,
          [MANIFEST_ENV_KEYS.service]: reviewManifests.service,
          [MANIFEST_ENV_KEYS.ingress]: reviewManifests.ingress,
        },
      });

      const result = await deployPipelineMutation.mutateAsync({
        pipelineId: pipeline.id,
      });
      navigate(
        `/cicd/pipelines/${pipeline.id}/logs?deploymentId=${result.deploymentId}`,
      );
    } catch (error) {
      setSubmitError(createPipelineErrorMessage(error));
    }
  };

  return (
    <div>
      <Breadcrumb
        items={[
          { label: t("sidebar.cicdList", "CI/CD List"), path: "/cicd/list" },
          { label: t("developerDeployPage.title", "Pipeline Setup") },
        ]}
      />

      <div className="mb-7 flex items-center gap-2.5">
        <div className="flex h-[var(--icon-size)] w-[var(--icon-size)] items-center justify-center rounded-[var(--icon-radius)] bg-[rgba(99,102,241,0.15)] text-[#818cf8]">
          <Rocket size={18} />
        </div>
        <div>
          <h1 className="m-0 text-[22px] font-extrabold text-[var(--color-text-primary)]">
            {t("developerDeployPage.title", "Pipeline Setup")}
          </h1>
          <p className="m-0 mt-0.5 text-[13px] text-[var(--color-text-secondary)]">
            {t(
              "developerDeployPage.description",
              "Configure your application pipeline before deployment.",
            )}
          </p>
        </div>
      </div>

      <div className="mb-6 flex flex-wrap items-center gap-1">
        {([1, 2, 3, 4, 5, 6] as Step[]).map((step, index) => (
          <div key={step} className="flex items-center gap-1">
            <button
              type="button"
              aria-label={t(
                `developerDeployPage.steps.${step}`,
                STEP_LABEL_DEFAULTS[step],
              )}
              onClick={() => scrollToStep(step)}
              className="flex cursor-pointer items-center gap-1.5 rounded-md border-none bg-none px-1.5 py-1"
            >
              <span
                className={cn(
                  "flex h-[22px] w-[22px] shrink-0 items-center justify-center rounded-full text-[11px] font-bold",
                  step === activeStep
                    ? "bg-[#6366f1] text-white"
                    : step < activeStep
                      ? "bg-[rgba(34,197,94,0.3)] text-[#22c55e]"
                      : "bg-[rgba(255,255,255,0.08)] text-[var(--color-text-secondary)]",
                )}
              >
                {step}
              </span>
              <span
                className={cn(
                  "text-[13px]",
                  step === activeStep
                    ? "font-semibold text-[var(--color-text-primary)]"
                    : "text-[var(--color-text-secondary)]",
                )}
              >
                {t(
                  `developerDeployPage.steps.${step}`,
                  STEP_LABEL_DEFAULTS[step],
                )}
              </span>
            </button>
            {index < 5 && (
              <ChevronRight
                size={14}
                className="shrink-0 text-[var(--color-text-secondary)]"
              />
            )}
          </div>
        ))}
      </div>

      <div
        ref={(node) => {
          sectionRefs.current[1] = node;
        }}
        id="pipeline-step-basic-info"
        className={cn(sectionClassName, "mb-5 scroll-mt-6")}
      >
        <StepSection
          title={t("developerDeployPage.sections.appName", "Enter App Name")}
        >
          <label htmlFor="deploy-app-name" className={labelStyleClass}>
            {t("developerDeployPage.form.appName", "App Name")}
            <RequiredDot />
          </label>
          <Input
            id="deploy-app-name"
            placeholder="my-awesome-app"
            value={form.appName}
            onChange={(event) => setField("appName", event.target.value)}
          />
          {errors.appName && (
            <span className="text-xs text-[#ef4444]">
              {errors.appName.message}
            </span>
          )}
          <p className="mb-0 mt-1.5 text-xs text-[var(--color-text-secondary)]">
            {t(
              "developerDeployPage.appNameRule",
              "Only lowercase letters, numbers, and hyphens are allowed.",
            )}
          </p>
          <div className="mt-6 border-t border-[var(--color-border-default)] pt-5">
            <p className={cn(labelStyleClass, "mb-3")}>
              {t("developerDeployPage.form.capabilities", "Capabilities")}
              <RequiredDot />
            </p>
            <div className="mb-5 flex flex-wrap gap-5">
              {CAPABILITIES.map((capability) => {
                const disabled = DISABLED_CAPABILITIES.has(capability);
                return (
                  <label
                    key={capability}
                    className={cn(
                      "flex items-center gap-2 text-sm font-medium",
                      disabled
                        ? "text-[var(--color-text-secondary)] opacity-60"
                        : "text-[var(--color-text-primary)]",
                    )}
                  >
                    <input
                      type="checkbox"
                      checked={isSelected(capability)}
                      disabled={disabled}
                      onChange={() => toggleCapability(capability)}
                      className="h-4 w-4 accent-[#6366f1]"
                    />
                    {capability}
                  </label>
                );
              })}
            </div>
            <p className={cn(labelStyleClass, "mb-3")}>
              {t("developerDeployPage.form.phase", "Phase")}
              <RequiredDot />
            </p>
            <div className="flex flex-wrap gap-5">
              {PHASES.map((phase) => (
                <label
                  key={phase}
                  className="flex items-center gap-2 text-sm font-medium text-[var(--color-text-primary)]"
                >
                  <input
                    type="checkbox"
                    checked={selectedPhase === phase}
                    onChange={() => setSelectedPhase(phase)}
                    className="h-4 w-4 accent-[#6366f1]"
                  />
                  {phase}
                </label>
              ))}
            </div>
          </div>
        </StepSection>
      </div>

      <div
        ref={(node) => {
          sectionRefs.current[2] = node;
          sectionRefs.current[3] = node;
        }}
        id="pipeline-step-configuration"
        className={cn(sectionClassName, "mb-5 scroll-mt-6")}
      >
        <StepSection
          title={
            <>
              <span className="rounded-md bg-[rgba(99,102,241,0.14)] px-2 py-0.5 text-[11px] font-extrabold uppercase tracking-[0.04em] text-[#a5b4fc]">
                CI
              </span>
              <span>
                {t(
                  "developerDeployPage.sections.buildConfiguration",
                  "Build Configuration",
                )}
              </span>
            </>
          }
        >
          <div className="flex flex-col gap-6">
            <div>
              <label htmlFor="deploy-stack" className={labelStyleClass}>
                {t("developerDeployPage.form.stackRequired", "Stack 선택")}
                <RequiredDot />
              </label>
              <NativeSelect
                id="deploy-stack"
                value={selectedStackId}
                onChange={(event) => {
                  setSelectedStackId(event.target.value);
                  setRepoName(DEFAULT_REPOSITORY_PATH);
                  setBuildRepositoryPath(DEFAULT_DOCKERFILE_REPOSITORY_PATH);
                  setImageRegistryPath(DEFAULT_IMAGE_REGISTRY_PATH);
                  setValue("manifestPath", DEFAULT_DEPLOY_YAML_PATH, {
                    shouldValidate: true,
                    shouldDirty: true,
                  });
                }}
                className="w-full"
              >
                {stacks.length === 0 && (
                  <option value="">
                    {t("developerDeployPage.form.manualInput", "Manual Input")}
                  </option>
                )}
                {stacks.map((stack) => (
                  <option key={stack.id} value={stack.id}>
                    {stack.name}
                  </option>
                ))}
              </NativeSelect>
            </div>
            {isSelected("CI") && (
              <>
                <div className="flex flex-col gap-3">
                  <h3 className="m-0 text-sm font-semibold text-[var(--color-text-primary)]">
                    {t(
                      "developerDeployPage.sections.codeCheckout",
                      "1. Code Checkout",
                    )}
                  </h3>
                  <div>
                    <label
                      htmlFor="deploy-code-repository"
                      className={labelStyleClass}
                    >
                      {t(
                        "developerDeployPage.form.sourceRepository",
                        "Source Repository",
                      )}
                      <RequiredDot />
                    </label>
                    <div className="grid grid-cols-2 gap-2">
                      <Input value={stackGitBaseUrl} disabled />
                      <Input
                        id="deploy-code-repository"
                        placeholder={DEFAULT_REPOSITORY_PATH}
                        value={repoName}
                        onChange={(event) => setRepoName(event.target.value)}
                      />
                    </div>
                  </div>
                </div>

                <div className="border-t border-[var(--color-border-default)] pt-5">
                  <h3 className="mb-3 mt-0 text-sm font-semibold text-[var(--color-text-primary)]">
                    {t("developerDeployPage.sections.buildStage", "2. Build")}
                  </h3>
                  <div className="grid gap-3 md:grid-cols-3">
                    <div className="md:col-span-3">
                      <label
                        htmlFor="deploy-dockerfile-repository"
                        className={labelStyleClass}
                      >
                        {t(
                          "developerDeployPage.form.dockerfileRepository",
                          "Dockerfile Repository",
                        )}
                      </label>
                      {selectedStackId && stackGitBaseUrl ? (
                        <div className="grid grid-cols-2 gap-2">
                          <Input value={stackGitBaseUrl} disabled />
                          <Input
                            id="deploy-dockerfile-repository"
                            placeholder={DEFAULT_DOCKERFILE_REPOSITORY_PATH}
                            value={buildRepositoryPath}
                            onChange={(event) =>
                              setBuildRepositoryPath(event.target.value)
                            }
                          />
                        </div>
                      ) : (
                        <Input
                          id="deploy-dockerfile-repository"
                          value={repositoryUrl}
                          disabled
                          placeholder="https://github.com/org/repo.git"
                        />
                      )}
                    </div>
                  </div>
                </div>

                <div className="border-t border-[var(--color-border-default)] pt-5">
                  <h3 className="mb-3 mt-0 text-sm font-semibold text-[var(--color-text-primary)]">
                    {t(
                      "developerDeployPage.sections.imageRegistry",
                      "3. Image Registry",
                    )}
                  </h3>
                  <div>
                    <label
                      htmlFor="deploy-image-registry"
                      className={labelStyleClass}
                    >
                      {t(
                        "developerDeployPage.form.imageRegistry",
                        "Image Registry",
                      )}
                    </label>
                    {selectedStackId && imageRegistryBaseUrl ? (
                      <div className="grid grid-cols-2 gap-2">
                        <Input value={imageRegistryBaseUrl} disabled />
                        <Input
                          id="deploy-image-registry"
                          placeholder={DEFAULT_IMAGE_REGISTRY_PATH}
                          value={imageRegistryPath}
                          onChange={(event) =>
                            setImageRegistryPath(event.target.value)
                          }
                        />
                      </div>
                    ) : (
                      <Input
                        id="deploy-image-registry"
                        placeholder="https://gitlab.nullus-devsecops-stack.internal/owner/repo/branch/directory"
                        value={imageRegistryPath}
                        onChange={(event) =>
                          setImageRegistryPath(event.target.value)
                        }
                      />
                    )}
                  </div>
                </div>
              </>
            )}
          </div>
        </StepSection>
      </div>

      {isSelected("Test") && (
        <div
          ref={(node) => {
            sectionRefs.current[4] = node;
          }}
          id="pipeline-step-test"
          className={cn(sectionClassName, "mb-5 scroll-mt-6")}
        >
          <StepSection title={t("developerDeployPage.sections.test", "Test")}>
            <p className="m-0 text-sm text-[var(--color-text-secondary)]">
              {t(
                "developerDeployPage.sections.testDescription",
                "Test execution is configured by the selected CI/CD template.",
              )}
            </p>
          </StepSection>
        </div>
      )}

      {isSelected("Security") && (
        <div
          ref={(node) => {
            sectionRefs.current[5] = node;
          }}
          id="pipeline-step-security"
          className={cn(sectionClassName, "mb-5 scroll-mt-6")}
        >
          <StepSection
            title={t("developerDeployPage.sections.security", "Security")}
          >
            <p className="m-0 text-sm text-[var(--color-text-secondary)]">
              {t(
                "developerDeployPage.sections.securityDescription",
                "Security checks are configured by the selected CI/CD template.",
              )}
            </p>
          </StepSection>
        </div>
      )}

      <div
        ref={(node) => {
          sectionRefs.current[6] = node;
        }}
        id="pipeline-step-deploy"
        className="scroll-mt-6"
      >
        {!isSelected("CD") ? (
          <div className={sectionClassName}>
            <StepSection title={t("developerDeployPage.steps.6", "Create")}>
              <p className="m-0 text-sm text-[var(--color-text-secondary)]">
                {t(
                  "developerDeployPage.sections.cdNotSelected",
                  "Select CD in Basic Info to configure deployment.",
                )}
              </p>
            </StepSection>
          </div>
        ) : (
          <div className={sectionClassName}>
            <StepSection
              title={
                <>
                  <span className="rounded-md bg-[rgba(34,197,94,0.14)] px-2 py-0.5 text-[11px] font-extrabold uppercase tracking-[0.04em] text-[#86efac]">
                    CD
                  </span>
                  <span>
                    {t(
                      "developerDeployPage.sections.deployConfiguration",
                      "Deploy Configuration",
                    )}
                  </span>
                </>
              }
            >
              <div className="grid grid-cols-1 items-start gap-6 xl:grid-cols-[minmax(400px,1fr)_minmax(420px,1fr)]">
                <div className="flex flex-col gap-6">
                  <section>
                    <div>
                      <label
                        htmlFor="deploy-manifest-path"
                        className={labelStyleClass}
                      >
                        {t(
                          "developerDeployPage.form.deployYamlRepository",
                          "Deploy YAML Repository",
                        )}
                      </label>
                      {selectedStackId && artifactRepositoryBaseUrl ? (
                        <div className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] gap-2">
                          <Input value={artifactRepositoryBaseUrl} disabled />
                          <Input
                            id="deploy-manifest-path"
                            placeholder={DEFAULT_DEPLOY_YAML_PATH}
                            value={form.manifestPath}
                            onChange={(event) =>
                              setField("manifestPath", event.target.value)
                            }
                          />
                          <Button
                            type="button"
                            variant="outline"
                            size="md"
                            loading={isLoadingManifests}
                            onClick={loadDeployYamlDirectory}
                            aria-label="Reload Deploy YAML files"
                            title="Reload Deploy YAML files"
                          >
                            <RefreshCw size={14} />
                          </Button>
                        </div>
                      ) : (
                        <div className="grid grid-cols-[minmax(0,1fr)_auto] gap-2">
                          <Input
                            id="deploy-manifest-path"
                            placeholder="https://artifactory.example.com/root/spring-sample/deploy"
                            value={form.configRepositoryUrl}
                            onChange={(event) =>
                              setField(
                                "configRepositoryUrl",
                                event.target.value,
                              )
                            }
                          />
                          <Button
                            type="button"
                            variant="outline"
                            size="md"
                            loading={isLoadingManifests}
                            onClick={loadDeployYamlDirectory}
                            aria-label="Reload Deploy YAML files"
                            title="Reload Deploy YAML files"
                          >
                            <RefreshCw size={14} />
                          </Button>
                        </div>
                      )}
                      {manifestLoadError && (
                        <p className="mb-0 mt-2 text-xs text-[#f87171]">
                          {manifestLoadError}
                        </p>
                      )}
                    </div>
                  </section>

                  <section className="border-t border-[var(--color-border-default)] pt-5">
                    <h3 className="mb-3 mt-0 text-sm font-semibold text-[var(--color-text-primary)]">
                      {t(
                        "developerDeployPage.sections.clusterNamespace",
                        "Cluster & Namespace",
                      )}
                    </h3>
                    <div className="flex flex-col gap-3">
                      <div>
                        <label
                          htmlFor="deploy-cluster"
                          className={labelStyleClass}
                        >
                          {t("developerDeployPage.form.cluster", "Cluster")}
                          <RequiredDot />
                        </label>
                        <NativeSelect
                          id="deploy-cluster"
                          value={form.clusterId}
                          onChange={(event) => {
                            setField("clusterId", event.target.value);
                            setCreateNewNamespace(false);
                            setField("namespace", "default");
                          }}
                          className="w-full"
                        >
                          {clusters.map((cluster) => (
                            <option key={cluster.id} value={cluster.id}>
                              {cluster.name}
                            </option>
                          ))}
                        </NativeSelect>
                        {errors.clusterId && (
                          <span className="text-xs text-[#ef4444]">
                            {errors.clusterId.message}
                          </span>
                        )}
                      </div>
                      <div>
                        <label
                          htmlFor="deploy-namespace"
                          className={labelStyleClass}
                        >
                          {t("developerDeployPage.form.namespace", "Namespace")}
                          <RequiredDot />
                        </label>
                        <NativeSelect
                          id="deploy-namespace"
                          value={
                            createNewNamespace
                              ? "__new__"
                              : form.namespace || "default"
                          }
                          onChange={(event) => {
                            if (event.target.value === "__new__") {
                              setCreateNewNamespace(true);
                              setField("namespace", "");
                            } else {
                              setCreateNewNamespace(false);
                              setField("namespace", event.target.value);
                            }
                          }}
                          className="w-full"
                        >
                          {namespaceOptions.map((namespace) => (
                            <option key={namespace} value={namespace}>
                              {namespace}
                            </option>
                          ))}
                          <option value="__new__">
                            {t(
                              "developerDeployPage.form.newNamespace",
                              "New Namespace",
                            )}
                          </option>
                        </NativeSelect>
                        {createNewNamespace && (
                          <Input
                            className="mt-2"
                            placeholder={t(
                              "developerDeployPage.form.newNamespacePlaceholder",
                              "my-namespace",
                            )}
                            value={form.namespace}
                            onChange={(event) =>
                              setField("namespace", event.target.value)
                            }
                          />
                        )}
                        {errors.namespace && (
                          <span className="text-xs text-[#ef4444]">
                            {errors.namespace.message}
                          </span>
                        )}
                      </div>
                      <div>
                        <label
                          htmlFor="deploy-service-url"
                          className={labelStyleClass}
                        >
                          {t(
                            "developerDeployPage.form.serviceUrl",
                            "Service URL",
                          )}
                        </label>
                        <Input
                          id="deploy-service-url"
                          placeholder={t(
                            "developerDeployPage.form.serviceUrlPlaceholder",
                            "app.example.com",
                          )}
                          value={form.serviceUrl}
                          onChange={(event) =>
                            setField("serviceUrl", event.target.value)
                          }
                        />
                      </div>
                    </div>
                  </section>

                  <section className="border-t border-[var(--color-border-default)] pt-5">
                    <h3 className="mb-3 mt-0 text-sm font-semibold text-[var(--color-text-primary)]">
                      {t(
                        "developerDeployPage.sections.resources",
                        "Resource Configuration",
                      )}
                    </h3>
                    <div className="grid grid-cols-2 gap-3">
                      <div className="col-span-2">
                        <label
                          htmlFor="deploy-replicas"
                          className={labelStyleClass}
                        >
                          Replicas
                          <RequiredDot />
                        </label>
                        <Input
                          id="deploy-replicas"
                          type="number"
                          min={1}
                          max={10}
                          value={form.replicas}
                          onChange={(event) =>
                            setField("replicas", Number(event.target.value))
                          }
                        />
                        {errors.replicas && (
                          <span className="text-xs text-[#ef4444]">
                            {errors.replicas.message}
                          </span>
                        )}
                      </div>
                      <div>
                        <label
                          htmlFor="deploy-cpu-request"
                          className={labelStyleClass}
                        >
                          CPU Request
                          <RequiredDot />
                        </label>
                        <Input
                          id="deploy-cpu-request"
                          value={form.cpuRequest}
                          onChange={(event) =>
                            setField("cpuRequest", event.target.value)
                          }
                        />
                      </div>
                      <div>
                        <label
                          htmlFor="deploy-cpu-limit"
                          className={labelStyleClass}
                        >
                          CPU Limit
                          <RequiredDot />
                        </label>
                        <Input
                          id="deploy-cpu-limit"
                          value={form.cpuLimit}
                          onChange={(event) =>
                            setField("cpuLimit", event.target.value)
                          }
                        />
                      </div>
                      <div>
                        <label
                          htmlFor="deploy-memory-request"
                          className={labelStyleClass}
                        >
                          Memory Request
                          <RequiredDot />
                        </label>
                        <Input
                          id="deploy-memory-request"
                          value={form.memoryRequest}
                          onChange={(event) =>
                            setField("memoryRequest", event.target.value)
                          }
                        />
                      </div>
                      <div>
                        <label
                          htmlFor="deploy-memory-limit"
                          className={labelStyleClass}
                        >
                          Memory Limit
                          <RequiredDot />
                        </label>
                        <Input
                          id="deploy-memory-limit"
                          value={form.memoryLimit}
                          onChange={(event) =>
                            setField("memoryLimit", event.target.value)
                          }
                        />
                      </div>
                    </div>
                  </section>

                  <section className="border-t border-[var(--color-border-default)] pt-5">
                    <h3 className="mb-3 mt-0 text-sm font-semibold text-[var(--color-text-primary)]">
                      {t(
                        "developerDeployPage.sections.envVars",
                        "Environment Variables",
                      )}
                    </h3>
                    <div className="flex flex-col gap-2">
                      {fields.map((field, index) => (
                        <div key={field.id}>
                          <div className="flex items-center gap-2">
                            <Input
                              placeholder="KEY"
                              {...register(`envVars.${index}.key`)}
                              className="flex-1 font-mono text-[13px]"
                            />
                            <Input
                              placeholder="value"
                              {...register(`envVars.${index}.value`)}
                              className="flex-[2] font-mono text-[13px]"
                            />
                            <button
                              type="button"
                              onClick={() => remove(index)}
                              className="shrink-0 cursor-pointer border-none bg-none p-1 text-[#f87171]"
                            >
                              <Trash2 size={14} />
                            </button>
                          </div>
                          {errors.envVars?.[index]?.key?.message && (
                            <span className="text-xs text-[#ef4444]">
                              {errors.envVars[index]?.key?.message}
                            </span>
                          )}
                        </div>
                      ))}
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => append({ key: "", value: "" })}
                        className="mt-1 self-start"
                        type="button"
                      >
                        <Plus size={13} />
                        {t(
                          "developerDeployPage.actions.addVariable",
                          "Add Variable",
                        )}
                      </Button>
                    </div>
                  </section>
                </div>

                <div className="xl:sticky xl:top-6">
                  {canReview ? (
                    <section>
                      <h3 className="mb-3 mt-0 text-sm font-semibold text-[var(--color-text-primary)]">
                        {t(
                          "developerDeployPage.sections.manifest",
                          "Review Manifest",
                        )}
                      </h3>
                      <p className="mb-4 mt-0 text-xs text-[var(--color-text-secondary)]">
                        {t(
                          "developerDeployPage.manifestDescription",
                          "Review generated manifests before creating the pipeline.",
                        )}
                      </p>
                      <div className="flex flex-col gap-4">
                        <p className="m-0 text-sm font-medium text-[var(--color-text-primary)]">
                          {t(
                            "developerDeployPage.manifestTypes.deployment",
                            "Deployment",
                          )}
                        </p>
                        <CodePreview
                          code={reviewManifests.deployment}
                          language="yaml"
                          title={`${form.appName}-deployment.yaml`}
                          maxHeight="380px"
                        />
                        {(["service", "ingress"] as const).map((manifest) => (
                          <div key={manifest} className="flex flex-col gap-2">
                            <p className="m-0 text-sm font-medium text-[var(--color-text-primary)]">
                              {t(
                                `developerDeployPage.manifestTypes.${manifest}`,
                                manifest === "service" ? "Service" : "Ingress",
                              )}
                            </p>
                            <CodePreview
                              code={reviewManifests[manifest]}
                              language="yaml"
                              title={`${form.appName}-${manifest}.yaml`}
                              maxHeight={
                                manifest === "service" ? "260px" : "320px"
                              }
                            />
                          </div>
                        ))}
                      </div>
                      <div className="mt-6 flex justify-end">
                        <Button
                          variant="primary"
                          size="md"
                          type="button"
                          loading={
                            createPipelineMutation.isPending ||
                            deployPipelineMutation.isPending
                          }
                          disabled={
                            isSubmitting ||
                            !canReview ||
                            createPipelineMutation.isPending ||
                            deployPipelineMutation.isPending
                          }
                          onClick={handleSubmit((data) => {
                            setValue("gitUrl", gitUrl, {
                              shouldValidate: true,
                              shouldDirty: true,
                            });
                            return onSubmit({ ...data, gitUrl });
                          })}
                        >
                          <Rocket size={14} />
                          {t("developerDeployPage.actions.execute", "Execute")}
                        </Button>
                      </div>
                      {submitError && (
                        <p className="mt-3 text-right text-xs text-[#f87171]">
                          {submitError}
                        </p>
                      )}
                    </section>
                  ) : (
                    <p className="m-0 text-sm text-[var(--color-text-secondary)]">
                      {t(
                        "developerDeployPage.reviewPending",
                        "Complete required fields to preview deployment manifests.",
                      )}
                    </p>
                  )}
                </div>
              </div>
            </StepSection>
          </div>
        )}
      </div>
    </div>
  );
}
