import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { renderWithProviders } from "../../../__tests__/test-utils";
import type { UpgradeBundle } from "../../stack/api/stack-api-types";
import {
  UpgradeBundlesPanel,
  nextBundleStatuses,
} from "./upgrade-bundles-panel";
import { parseCVEList } from "./upgrade-bundle-register-modal";

const registerMutate = vi.hoisted(() => vi.fn());
const changeMutate = vi.hoisted(() => vi.fn());
const mockBundles = vi.hoisted(() => ({
  current: [] as UpgradeBundle[],
  isLoading: false,
  isError: false,
}));

vi.mock("../../stack/api/stack-api", () => ({
  useUpgradeBundles: () => ({
    data: { items: mockBundles.current },
    isLoading: mockBundles.isLoading,
    isError: mockBundles.isError,
  }),
  useRegisterUpgradeBundle: () => ({
    mutateAsync: registerMutate,
    isPending: false,
  }),
  useChangeUpgradeBundleStatus: () => ({
    mutateAsync: changeMutate,
    isPending: false,
  }),
}));

function bundle(overrides: Partial<UpgradeBundle> = {}): UpgradeBundle {
  return {
    id: "argocd-7.7.16-7.7.22",
    tool: "argocd",
    release_name: "argo-cd",
    chart_name: "argo-cd",
    repo_url: "https://argoproj.github.io/argo-helm",
    source_chart_version: "7.7.16",
    source_app_version: "v2.13.3",
    target_chart_version: "7.7.22",
    target_app_version: "v2.13.4",
    risk_level: "low",
    expected_disruption: "API server restarts for a minute",
    requires_backup: true,
    supported_architectures: ["amd64", "arm64"],
    status: "verified",
    fixes_cves: ["CVE-2025-12345"],
    created_at: "2026-10-25T09:00:00Z",
    ...overrides,
  };
}

describe("UpgradeBundlesPanel", () => {
  beforeEach(() => {
    registerMutate.mockReset();
    changeMutate.mockReset();
    mockBundles.current = [];
    mockBundles.isLoading = false;
    mockBundles.isError = false;
  });

  it("shows an empty state when no bundle is registered", () => {
    renderWithProviders(
      <UpgradeBundlesPanel registerOpen={false} onRegisterClose={() => {}} />,
    );
    expect(screen.getByTestId("upgrade-bundles-empty")).toBeInTheDocument();
    expect(
      screen.getByText("No upgrade bundles registered"),
    ).toBeInTheDocument();
  });

  it("lists bundles with path, status and the actions allowed from that status", () => {
    mockBundles.current = [
      bundle(),
      bundle({
        id: "argocd-7.7.22-7.7.23",
        source_chart_version: "7.7.22",
        target_chart_version: "7.7.23",
        status: "draft",
        fixes_cves: [],
      }),
    ];
    renderWithProviders(
      <UpgradeBundlesPanel registerOpen={false} onRegisterClose={() => {}} />,
    );

    const verified = within(
      screen.getByTestId("upgrade-bundle-argocd-7.7.16-7.7.22"),
    );
    expect(verified.getByText(/7\.7\.16 \(v2\.13\.3\)/)).toBeInTheDocument();
    expect(verified.getByText("7.7.22 (v2.13.4)")).toBeInTheDocument();
    expect(verified.getByText("CVE-2025-12345")).toBeInTheDocument();
    expect(verified.getByLabelText("status verified")).toHaveTextContent(
      "Verified",
    );
    expect(
      verified.getByRole("button", { name: "Deprecate" }),
    ).toBeInTheDocument();
    expect(
      verified.getByRole("button", { name: "Revoke" }),
    ).toBeInTheDocument();
    expect(
      verified.queryByRole("button", { name: "Mark verified" }),
    ).not.toBeInTheDocument();

    const draft = within(
      screen.getByTestId("upgrade-bundle-argocd-7.7.22-7.7.23"),
    );
    expect(draft.getByLabelText("status draft")).toHaveTextContent("Draft");
    expect(
      draft.getByRole("button", { name: "Mark verified" }),
    ).toBeInTheDocument();
    expect(
      draft.queryByRole("button", { name: "Deprecate" }),
    ).not.toBeInTheDocument();
  });

  it("revokes a bundle after confirmation", async () => {
    mockBundles.current = [bundle()];
    changeMutate.mockResolvedValue(bundle({ status: "revoked" }));
    renderWithProviders(
      <UpgradeBundlesPanel registerOpen={false} onRegisterClose={() => {}} />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Revoke" }));
    const dialog = screen.getByRole("dialog", { name: "Change bundle status" });
    expect(within(dialog).getByText(/cannot be undone/i)).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: "Change" }));

    await waitFor(() =>
      expect(changeMutate).toHaveBeenCalledWith({
        id: "argocd-7.7.16-7.7.22",
        status: "revoked",
        rehearsalConfirmed: false,
      }),
    );
    await waitFor(() =>
      expect(
        screen.queryByRole("dialog", { name: "Change bundle status" }),
      ).not.toBeInTheDocument(),
    );
  });

  it("requires the rehearsal confirmation before marking a draft verified", async () => {
    mockBundles.current = [bundle({ status: "draft" })];
    changeMutate.mockResolvedValue(bundle({ status: "verified" }));
    renderWithProviders(
      <UpgradeBundlesPanel registerOpen={false} onRegisterClose={() => {}} />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Mark verified" }));
    const dialog = screen.getByRole("dialog", { name: "Change bundle status" });
    const confirm = within(dialog).getByRole("button", { name: "Change" });
    expect(confirm).toBeDisabled();
    fireEvent.click(within(dialog).getByRole("checkbox"));
    expect(confirm).toBeEnabled();
    fireEvent.click(confirm);

    await waitFor(() =>
      expect(changeMutate).toHaveBeenCalledWith({
        id: "argocd-7.7.16-7.7.22",
        status: "verified",
        rehearsalConfirmed: true,
      }),
    );
  });

  it("registers a verified bundle only with the rehearsal confirmation", async () => {
    registerMutate.mockResolvedValue(bundle());
    const onClose = vi.fn();
    renderWithProviders(
      <UpgradeBundlesPanel registerOpen onRegisterClose={onClose} />,
    );

    const dialog = screen.getByRole("dialog", {
      name: "Register upgrade bundle",
    });
    const d = within(dialog);
    fireEvent.change(d.getByLabelText("Source chart version"), {
      target: { value: "7.7.16" },
    });
    fireEvent.change(d.getByLabelText("Source app version"), {
      target: { value: "v2.13.3" },
    });
    fireEvent.change(d.getByLabelText("Target chart version"), {
      target: { value: "7.7.22" },
    });
    fireEvent.change(d.getByLabelText("Target app version"), {
      target: { value: "v2.13.4" },
    });
    fireEvent.change(d.getByLabelText("Expected disruption"), {
      target: { value: "API server restarts" },
    });
    fireEvent.change(d.getByLabelText(/Fixed CVEs/), {
      target: { value: "cve-2025-12345, CVE-2025-12345 CVE-2024-1234" },
    });

    fireEvent.click(d.getByRole("button", { name: "Register" }));
    expect(await d.findByRole("alert")).toHaveTextContent(/rehearsal/i);
    expect(registerMutate).not.toHaveBeenCalled();

    fireEvent.click(
      d.getByLabelText("I rehearsed this path on a real cluster"),
    );
    fireEvent.click(d.getByRole("button", { name: "Register" }));

    await waitFor(() => expect(registerMutate).toHaveBeenCalledTimes(1));
    expect(registerMutate).toHaveBeenCalledWith({
      tool: "argocd",
      release_name: "argo-cd",
      chart_name: "argo-cd",
      repo_url: "https://argoproj.github.io/argo-helm",
      source_chart_version: "7.7.16",
      source_app_version: "v2.13.3",
      target_chart_version: "7.7.22",
      target_app_version: "v2.13.4",
      risk_level: "low",
      expected_disruption: "API server restarts",
      release_notes: "",
      requires_backup: true,
      supported_architectures: ["amd64", "arm64"],
      fixes_cves: ["CVE-2025-12345", "CVE-2024-1234"],
      status: "verified",
      rehearsal_confirmed: true,
    });
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });

  it("blocks registration when the target equals the source and keeps server errors visible", async () => {
    registerMutate.mockRejectedValue({
      response: { data: { error: { message: "same path exists" } } },
    });
    renderWithProviders(
      <UpgradeBundlesPanel registerOpen onRegisterClose={() => {}} />,
    );
    const d = within(
      screen.getByRole("dialog", { name: "Register upgrade bundle" }),
    );

    fireEvent.change(d.getByLabelText("Source chart version"), {
      target: { value: "7.7.16" },
    });
    fireEvent.change(d.getByLabelText("Target chart version"), {
      target: { value: "7.7.16" },
    });
    fireEvent.click(
      d.getByLabelText("I rehearsed this path on a real cluster"),
    );
    fireEvent.click(d.getByRole("button", { name: "Register" }));
    expect(
      await d.findByText("Target must differ from the source version."),
    ).toBeInTheDocument();
    expect(registerMutate).not.toHaveBeenCalled();

    fireEvent.change(d.getByLabelText("Target chart version"), {
      target: { value: "7.7.22" },
    });
    fireEvent.click(d.getByRole("button", { name: "Register" }));
    expect(await d.findByText("same path exists")).toBeInTheDocument();
  });
});

describe("bundle helpers", () => {
  it("maps every status to the transitions the domain allows", () => {
    expect(nextBundleStatuses("draft")).toEqual(["verified", "revoked"]);
    expect(nextBundleStatuses("verified")).toEqual(["deprecated", "revoked"]);
    expect(nextBundleStatuses("deprecated")).toEqual(["verified", "revoked"]);
    expect(nextBundleStatuses("revoked")).toEqual([]);
    expect(nextBundleStatuses(undefined)).toEqual([]);
  });

  it("parses CVE lists separated by commas or spaces, uppercased and deduplicated", () => {
    expect(
      parseCVEList(" cve-2025-1, CVE-2025-1;CVE-2024-99\nCVE-2023-5 "),
    ).toEqual(["CVE-2025-1", "CVE-2024-99", "CVE-2023-5"]);
    expect(parseCVEList("")).toEqual([]);
  });
});
