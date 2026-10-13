import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  buildCreatePayload,
  buildUpdatePayload,
  channelTarget,
  emptyChannelForm,
  formFromChannel,
  notificationApiCalls,
  notificationQueryKeys,
  parseRecipients,
  type NotificationChannel,
} from "./notification-api";

const { mockGet, mockPost, mockPatch, mockDelete } = vi.hoisted(() => ({
  mockGet: vi.fn(),
  mockPost: vi.fn(),
  mockPatch: vi.fn(),
  mockDelete: vi.fn(),
}));

vi.mock("../../../lib/api", () => ({
  api: { get: mockGet, post: mockPost, patch: mockPatch, delete: mockDelete },
}));

const telegram: NotificationChannel = {
  id: "ch-1",
  name: "운영 텔레그램",
  type: "telegram",
  settings: { chat_id: "-100123" },
  secret_set: true,
  secret_hint: "bot 123456789",
  events: ["backup_failed"],
  active: true,
  created_at: "2026-10-13T00:00:00Z",
  updated_at: "2026-10-13T00:00:00Z",
};

describe("notificationApiCalls", () => {
  beforeEach(() => vi.clearAllMocks());

  it("should call the admin notification endpoints", async () => {
    mockGet.mockResolvedValue({ data: { items: [telegram], total: 1 } });
    mockPost.mockResolvedValue({ data: { status: "sent" } });
    mockPatch.mockResolvedValue({ data: telegram });
    mockDelete.mockResolvedValue({ data: undefined });

    expect(await notificationApiCalls.listChannels()).toEqual([telegram]);
    expect(mockGet).toHaveBeenCalledWith("/admin/notifications/channels");

    await notificationApiCalls.testChannel("ch-1");
    expect(mockPost).toHaveBeenCalledWith(
      "/admin/notifications/channels/ch-1/test",
    );

    await notificationApiCalls.updateChannel("ch-1", { active: false });
    expect(mockPatch).toHaveBeenCalledWith(
      "/admin/notifications/channels/ch-1",
      { active: false },
    );

    await notificationApiCalls.deleteChannel("ch-1");
    expect(mockDelete).toHaveBeenCalledWith(
      "/admin/notifications/channels/ch-1",
    );

    mockGet.mockResolvedValue({ data: { items: [], total: 0 } });
    await notificationApiCalls.listHistory();
    expect(mockGet).toHaveBeenLastCalledWith("/admin/notifications/history", {
      params: { limit: 30 },
    });
  });

  it("should key history separately so a test send can refresh it", () => {
    expect(notificationQueryKeys.channels()).toEqual([
      "admin",
      "notifications",
      "channels",
    ]);
    expect(notificationQueryKeys.history()).toEqual([
      "admin",
      "notifications",
      "history",
    ]);
  });
});

describe("parseRecipients", () => {
  it("should split by comma, semicolon and newline and drop blanks", () => {
    expect(
      parseRecipients("ops@example.com, dba@example.com\n\n;sec@example.com "),
    ).toEqual(["ops@example.com", "dba@example.com", "sec@example.com"]);
  });
});

describe("buildCreatePayload", () => {
  it("should send only the fields of the chosen channel type", () => {
    const form = {
      ...emptyChannelForm(["backup_failed", "backup_partial"]),
      type: "email" as const,
      name: "운영 메일",
      chatId: "-100123",
      smtpHost: "smtp.example.com",
      smtpPort: "587",
      smtpSecurity: "starttls" as const,
      username: "nullus",
      secret: "app-password",
      from: "Nullus <nullus@example.com>",
      to: "ops@example.com, dba@example.com",
    };
    expect(buildCreatePayload(form)).toEqual({
      name: "운영 메일",
      type: "email",
      settings: {
        smtp_host: "smtp.example.com",
        smtp_port: 587,
        smtp_security: "starttls",
        username: "nullus",
        from: "Nullus <nullus@example.com>",
        to: ["ops@example.com", "dba@example.com"],
      },
      secret: "app-password",
      events: ["backup_failed", "backup_partial"],
      active: true,
    });
  });

  it("should leave the port to the server when blank", () => {
    const form = {
      ...emptyChannelForm([]),
      type: "email" as const,
      smtpPort: "",
    };
    expect(buildCreatePayload(form).settings).not.toHaveProperty("smtp_port");
  });

  it("should send telegram chat id and slack nothing but the secret", () => {
    const tg = {
      ...emptyChannelForm(["backup_failed"]),
      type: "telegram" as const,
      chatId: " -100123 ",
      secret: "tok",
    };
    expect(buildCreatePayload(tg).settings).toEqual({ chat_id: "-100123" });
    const slack = {
      ...emptyChannelForm(["backup_failed"]),
      type: "slack" as const,
      chatId: "x",
      secret: "https://hooks.slack.com/x",
    };
    expect(buildCreatePayload(slack).settings).toEqual({});
  });
});

describe("buildUpdatePayload", () => {
  it("should not send an empty secret so the stored one is kept", () => {
    const form = formFromChannel(telegram);
    expect(form.secret).toBe("");
    const payload = buildUpdatePayload(form);
    expect(payload).not.toHaveProperty("secret");
    expect(payload.settings).toEqual({ chat_id: "-100123" });
    expect(payload.events).toEqual(["backup_failed"]);
  });

  it("should send a new secret when typed", () => {
    const form = { ...formFromChannel(telegram), secret: "987:new" };
    expect(buildUpdatePayload(form).secret).toBe("987:new");
  });
});

describe("channelTarget", () => {
  it("should show where each channel delivers without the secret", () => {
    expect(channelTarget(telegram)).toBe("chat -100123 · bot 123456789");
    expect(
      channelTarget({
        ...telegram,
        type: "email",
        settings: { smtp_host: "smtp.example.com", to: ["a@x.io", "b@x.io"] },
        secret_hint: "",
      }),
    ).toBe("a@x.io, b@x.io (smtp.example.com)");
    expect(
      channelTarget({
        ...telegram,
        type: "slack",
        settings: {},
        secret_hint: "hooks.slack.com …WXYZ",
      }),
    ).toBe("hooks.slack.com …WXYZ");
  });
});
