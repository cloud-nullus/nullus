import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { fireEvent, screen, within } from "@testing-library/react";
import { renderWithProviders } from "../../../__tests__/test-utils";
import { NotificationChannelsPage } from "./notification-channels-page";

const mocks = vi.hoisted(() => ({
  channels: vi.fn(),
  history: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  remove: vi.fn(),
  test: vi.fn(),
  toastSuccess: vi.fn(),
  toastError: vi.fn(),
}));

vi.mock("sonner", () => ({
  toast: { success: mocks.toastSuccess, error: mocks.toastError },
}));

vi.mock("../api/notification-api", async () => {
  const actual = await vi.importActual<
    typeof import("../api/notification-api")
  >("../api/notification-api");
  return {
    ...actual,
    useNotificationChannels: () => mocks.channels(),
    useNotificationHistory: () => mocks.history(),
    useNotificationEvents: () => ({
      data: [
        { type: "backup_failed", default_on: true, description: "" },
        { type: "backup_partial", default_on: true, description: "" },
        { type: "backup_succeeded", default_on: false, description: "" },
      ],
    }),
    useCreateNotificationChannel: () => ({
      mutate: mocks.create,
      isPending: false,
    }),
    useUpdateNotificationChannel: () => ({
      mutate: mocks.update,
      isPending: false,
    }),
    useDeleteNotificationChannel: () => ({
      mutate: mocks.remove,
      isPending: false,
    }),
    useTestNotificationChannel: () => ({
      mutate: mocks.test,
      isPending: false,
      variables: undefined,
    }),
  };
});

const telegram = {
  id: "ch-1",
  name: "운영 텔레그램",
  type: "telegram",
  settings: { chat_id: "-100123" },
  secret_set: true,
  secret_hint: "bot 123456789",
  events: ["backup_failed", "backup_partial"],
  active: true,
  created_at: "2026-10-13T00:00:00Z",
  updated_at: "2026-10-13T00:00:00Z",
};
/** 채널 표의 행. 이름은 발송 이력 표에도 나오므로 채널 표 안에서 찾는다. */
function channelRow(name: string): HTMLElement {
  const table = screen.getByRole("table", { name: "Notification Channels" });
  return within(table).getByText(name).closest("tr") as HTMLElement;
}

const mail = {
  ...telegram,
  id: "ch-2",
  name: "운영 메일",
  type: "email",
  settings: { smtp_host: "smtp.example.com", to: ["ops@example.com"] },
  secret_hint: "",
  events: ["backup_failed"],
  active: false,
};

describe("NotificationChannelsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.channels.mockReturnValue({ data: [telegram, mail] });
    mocks.history.mockReturnValue({
      data: [
        {
          id: "d1",
          channel_id: "ch-1",
          channel_name: "운영 텔레그램",
          channel_type: "telegram",
          event: "backup_failed",
          status: "failed",
          title: "백업 실패 — 플랫폼",
          attempts: 3,
          error: "HTTP 400: Bad Request: chat not found",
          created_at: "2026-10-13T01:00:00Z",
        },
      ],
    });
  });

  it("should list channels with where they deliver and what they receive", () => {
    renderWithProviders(<NotificationChannelsPage />);
    const row = channelRow("운영 텔레그램");
    expect(within(row).getByText("Telegram")).toBeInTheDocument();
    expect(
      within(row).getByText("chat -100123 · bot 123456789"),
    ).toBeInTheDocument();
    expect(within(row).getByText("Backup failed")).toBeInTheDocument();
    expect(within(row).getByText("On")).toBeInTheDocument();

    const mailRow = channelRow("운영 메일");
    expect(
      within(mailRow).getByText("ops@example.com (smtp.example.com)"),
    ).toBeInTheDocument();
    expect(within(mailRow).getByText("Off")).toBeInTheDocument();
  });

  it("should flag a channel whose stored secret cannot be read", () => {
    mocks.channels.mockReturnValue({
      data: [
        {
          ...telegram,
          secret_set: false,
          secret_hint: "",
          secret_unreadable: true,
        },
      ],
    });
    renderWithProviders(<NotificationChannelsPage />);
    expect(
      within(channelRow("운영 텔레그램")).getByText(
        /Stored secret cannot be read/,
      ),
    ).toBeInTheDocument();
  });

  it("should show an empty state", () => {
    mocks.channels.mockReturnValue({ data: [] });
    mocks.history.mockReturnValue({ data: [] });
    renderWithProviders(<NotificationChannelsPage />);
    expect(screen.getByText(/No channels yet/)).toBeInTheDocument();
    expect(screen.getByText("Nothing sent yet.")).toBeInTheDocument();
  });

  it("should send a test and report the result", () => {
    renderWithProviders(<NotificationChannelsPage />);
    const row = channelRow("운영 메일");
    fireEvent.click(within(row).getByRole("button", { name: "Send test" }));
    expect(mocks.test).toHaveBeenCalledWith("ch-2", expect.anything());

    const { onSuccess } = mocks.test.mock.calls[0][1];
    onSuccess({ status: "failed", error: "SMTP 연결 실패" });
    expect(mocks.toastError).toHaveBeenCalledWith(
      expect.stringContaining("SMTP 연결 실패"),
    );
    onSuccess({ status: "sent" });
    expect(mocks.toastSuccess).toHaveBeenCalled();
  });

  it("should delete after confirmation", () => {
    renderWithProviders(<NotificationChannelsPage />);
    const row = channelRow("운영 텔레그램");
    fireEvent.click(within(row).getByRole("button", { name: "Delete" }));
    expect(mocks.remove).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Delete channel" }));
    expect(mocks.remove).toHaveBeenCalledWith("ch-1", expect.anything());
  });

  it("should create a channel from the dialog", () => {
    renderWithProviders(<NotificationChannelsPage />);
    fireEvent.click(screen.getByRole("button", { name: "Add channel" }));
    fireEvent.click(screen.getByRole("radio", { name: "Slack" }));
    fireEvent.change(screen.getByLabelText("Name"), {
      target: { value: "slack #ops" },
    });
    fireEvent.change(screen.getByLabelText("Incoming Webhook URL"), {
      target: { value: "https://hooks.slack.com/services/T/B/X" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(mocks.create).toHaveBeenCalledWith(
      {
        name: "slack #ops",
        type: "slack",
        settings: {},
        secret: "https://hooks.slack.com/services/T/B/X",
        events: ["backup_failed", "backup_partial"],
        active: true,
      },
      expect.anything(),
    );
  });

  it("should edit a channel without sending the secret", () => {
    renderWithProviders(<NotificationChannelsPage />);
    const row = channelRow("운영 텔레그램");
    fireEvent.click(within(row).getByRole("button", { name: "Edit" }));
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(mocks.update).toHaveBeenCalledWith(
      {
        id: "ch-1",
        body: {
          name: "운영 텔레그램",
          settings: { chat_id: "-100123" },
          events: ["backup_failed", "backup_partial"],
          active: true,
        },
      },
      expect.anything(),
    );
  });

  it("should show delivery history with failures", () => {
    renderWithProviders(<NotificationChannelsPage />);
    const history = screen.getByRole("table", { name: "Recent deliveries" });
    expect(within(history).getByText("백업 실패 — 플랫폼")).toBeInTheDocument();
    expect(within(history).getByText("Failed")).toBeInTheDocument();
    expect(
      within(history).getByText("HTTP 400: Bad Request: chat not found"),
    ).toBeInTheDocument();
    expect(within(history).getByText("3")).toBeInTheDocument();
  });
});
