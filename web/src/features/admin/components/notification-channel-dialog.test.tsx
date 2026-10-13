import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { fireEvent, screen } from "@testing-library/react";
import { renderWithProviders } from "../../../__tests__/test-utils";
import { NotificationChannelDialog } from "./notification-channel-dialog";
import type { EventSpec, NotificationChannel } from "../api/notification-api";

const EVENTS: EventSpec[] = [
  { type: "backup_failed", default_on: true, description: "" },
  { type: "backup_partial", default_on: true, description: "" },
  { type: "backup_succeeded", default_on: false, description: "" },
];

function renderDialog(
  props: Partial<Parameters<typeof NotificationChannelDialog>[0]> = {},
) {
  const onSubmit = vi.fn();
  renderWithProviders(
    <NotificationChannelDialog
      open
      onClose={vi.fn()}
      events={EVENTS}
      onSubmit={onSubmit}
      submitting={false}
      {...props}
    />,
  );
  return { onSubmit };
}

const type = (label: RegExp | string, value: string) =>
  fireEvent.change(screen.getByLabelText(label), { target: { value } });

describe("NotificationChannelDialog", () => {
  it("should create a telegram channel with default events", () => {
    const { onSubmit } = renderDialog();
    expect(screen.getByRole("radio", { name: "Telegram" })).toHaveAttribute(
      "aria-checked",
      "true",
    );
    // 성공 알림은 기본으로 끈다.
    expect(
      screen.getByRole("checkbox", { name: "Backup failed" }),
    ).toBeChecked();
    expect(
      screen.getByRole("checkbox", { name: "Backup succeeded" }),
    ).not.toBeChecked();

    const save = screen.getByRole("button", { name: "Save" });
    expect(save).toBeDisabled();

    type("Name", "운영 텔레그램");
    type("Bot token", "123456789:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw0");
    type("Chat ID", "-100123");
    expect(save).toBeEnabled();
    fireEvent.click(save);

    expect(onSubmit).toHaveBeenCalledTimes(1);
    const form = onSubmit.mock.calls[0][0];
    expect(form).toMatchObject({
      name: "운영 텔레그램",
      type: "telegram",
      chatId: "-100123",
      secret: "123456789:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw0",
      events: ["backup_failed", "backup_partial"],
      active: true,
    });
    // 비밀 입력은 화면에 보이지 않는다.
    expect(screen.getByLabelText("Bot token")).toHaveAttribute(
      "type",
      "password",
    );
  });

  it("should show SMTP fields for email and require password only with a username", () => {
    const { onSubmit } = renderDialog();
    fireEvent.click(screen.getByRole("radio", { name: "Email (SMTP)" }));
    expect(screen.queryByLabelText("Chat ID")).not.toBeInTheDocument();

    type("Name", "운영 메일");
    type("SMTP server", "smtp.example.com");
    type("From", "Nullus <nullus@example.com>");
    type("To", "ops@example.com\ndba@example.com");
    const save = screen.getByRole("button", { name: "Save" });
    expect(save).toBeEnabled();

    type("Username (optional)", "nullus");
    expect(save).toBeDisabled();
    type("Password", "app-password");
    expect(save).toBeEnabled();

    fireEvent.click(screen.getByRole("radio", { name: "TLS" }));
    fireEvent.click(save);
    expect(onSubmit.mock.calls[0][0]).toMatchObject({
      type: "email",
      smtpHost: "smtp.example.com",
      smtpSecurity: "tls",
      username: "nullus",
      secret: "app-password",
      to: "ops@example.com\ndba@example.com",
    });
  });

  it("should ask only for the webhook URL for slack", () => {
    const { onSubmit } = renderDialog();
    fireEvent.click(screen.getByRole("radio", { name: "Slack" }));
    type("Name", "slack #ops");
    type("Incoming Webhook URL", "https://hooks.slack.com/services/T/B/X");
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(onSubmit.mock.calls[0][0]).toMatchObject({
      type: "slack",
      secret: "https://hooks.slack.com/services/T/B/X",
    });
  });

  it("should require at least one event", () => {
    renderDialog();
    type("Name", "x");
    type("Bot token", "1:a");
    type("Chat ID", "1");
    fireEvent.click(screen.getByRole("checkbox", { name: "Backup failed" }));
    fireEvent.click(
      screen.getByRole("checkbox", { name: "Backup partially succeeded" }),
    );
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
  });

  it("should edit without exposing or requiring the stored secret and lock the type", () => {
    const channel: NotificationChannel = {
      id: "ch-1",
      name: "운영 텔레그램",
      type: "telegram",
      settings: { chat_id: "-100123" },
      secret_set: true,
      secret_hint: "bot 123456789",
      events: ["backup_failed"],
      active: true,
      created_at: "",
      updated_at: "",
    };
    const { onSubmit } = renderDialog({ channel });
    expect(screen.getByRole("radio", { name: "Slack" })).toBeDisabled();
    expect(screen.getByLabelText("Bot token")).toHaveValue("");
    expect(screen.getByText(/bot 123456789/)).toBeInTheDocument();

    type("Name", "이름 바꿈");
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(onSubmit.mock.calls[0][0]).toMatchObject({
      name: "이름 바꿈",
      secret: "",
    });
  });

  it("should show the server error", () => {
    renderDialog({ error: "봇 토큰 형식이 아닙니다" });
    expect(screen.getByRole("alert")).toHaveTextContent(
      "봇 토큰 형식이 아닙니다",
    );
  });
});
