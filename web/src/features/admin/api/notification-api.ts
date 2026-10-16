import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../../../lib/api";

/**
 * 알림 채널 API (nullus-plan#63) — Telegram · 이메일(SMTP) · Slack.
 *
 * 서버는 채널의 비밀(봇 토큰 · 웹훅 URL · SMTP 비밀번호)을 돌려주지 않는다. secret_set 과
 * secret_hint 만 온다. 그래서 편집할 때 비밀 칸은 비어 있고, 비워 두면 서버가 기존 값을 지킨다.
 */
export type ChannelType = "telegram" | "email" | "slack";
export type SMTPSecurity = "starttls" | "tls" | "none";
export type NotificationEvent =
  | "backup_failed"
  | "backup_partial"
  | "backup_succeeded";
export type DeliveryStatus = "sent" | "failed";

export const CHANNEL_TYPES: ChannelType[] = ["telegram", "email", "slack"];

export interface ChannelSettings {
  chat_id?: string;
  smtp_host?: string;
  smtp_port?: number;
  smtp_security?: SMTPSecurity;
  username?: string;
  from?: string;
  to?: string[];
}

export interface NotificationChannel {
  id: string;
  name: string;
  type: ChannelType;
  settings: ChannelSettings;
  secret_set: boolean;
  secret_hint?: string;
  /** 저장된 비밀을 서버가 풀지 못했다(ENCRYPTION_KEY 교체). 다시 입력해야 보낸다. */
  secret_unreadable?: boolean;
  events: NotificationEvent[];
  active: boolean;
  created_at: string;
  updated_at: string;
}

export interface EventSpec {
  type: NotificationEvent;
  default_on: boolean;
  description: string;
}

export interface NotificationDelivery {
  id: string;
  channel_id?: string;
  channel_name: string;
  channel_type: ChannelType;
  /** 구독 이벤트 또는 'test'(테스트 전송). */
  event: string;
  status: DeliveryStatus;
  title: string;
  attempts: number;
  error?: string;
  created_at: string;
}

export interface CreateChannelRequest {
  name: string;
  type: ChannelType;
  settings: ChannelSettings;
  secret?: string;
  events: NotificationEvent[];
  active: boolean;
}

export type UpdateChannelRequest = Partial<Omit<CreateChannelRequest, "type">>;

/** 화면 입력. 숫자·목록도 입력 칸 그대로(문자열) 들고 있다가 보낼 때 바꾼다. */
export interface ChannelForm {
  name: string;
  type: ChannelType;
  secret: string;
  chatId: string;
  smtpHost: string;
  smtpPort: string;
  smtpSecurity: SMTPSecurity;
  username: string;
  from: string;
  to: string;
  events: NotificationEvent[];
  active: boolean;
}

export function emptyChannelForm(
  defaultEvents: NotificationEvent[],
): ChannelForm {
  return {
    name: "",
    type: "telegram",
    secret: "",
    chatId: "",
    smtpHost: "",
    smtpPort: "",
    smtpSecurity: "starttls",
    username: "",
    from: "",
    to: "",
    events: [...defaultEvents],
    active: true,
  };
}

export function formFromChannel(ch: NotificationChannel): ChannelForm {
  const s = ch.settings ?? {};
  return {
    name: ch.name,
    type: ch.type,
    secret: "",
    chatId: s.chat_id ?? "",
    smtpHost: s.smtp_host ?? "",
    smtpPort: s.smtp_port ? String(s.smtp_port) : "",
    smtpSecurity: s.smtp_security ?? "starttls",
    username: s.username ?? "",
    from: s.from ?? "",
    to: (s.to ?? []).join(", "),
    events: [...ch.events],
    active: ch.active,
  };
}

/** 받는 주소 입력(쉼표 · 세미콜론 · 줄바꿈 구분)을 목록으로. */
export function parseRecipients(raw: string): string[] {
  return raw
    .split(/[,;\n]/)
    .map((s) => s.trim())
    .filter(Boolean);
}

function settingsFromForm(form: ChannelForm): ChannelSettings {
  switch (form.type) {
    case "telegram":
      return { chat_id: form.chatId.trim() };
    case "email": {
      const settings: ChannelSettings = {
        smtp_host: form.smtpHost.trim(),
        smtp_security: form.smtpSecurity,
        username: form.username.trim(),
        from: form.from.trim(),
        to: parseRecipients(form.to),
      };
      const port = Number.parseInt(form.smtpPort, 10);
      if (Number.isFinite(port) && port > 0) settings.smtp_port = port;
      return settings;
    }
    default:
      return {};
  }
}

export function buildCreatePayload(form: ChannelForm): CreateChannelRequest {
  return {
    name: form.name.trim(),
    type: form.type,
    settings: settingsFromForm(form),
    secret: form.secret.trim(),
    events: form.events,
    active: form.active,
  };
}

/** 비밀 칸이 비어 있으면 보내지 않는다 — 서버가 저장된 값을 지킨다. */
export function buildUpdatePayload(form: ChannelForm): UpdateChannelRequest {
  const payload: UpdateChannelRequest = {
    name: form.name.trim(),
    settings: settingsFromForm(form),
    events: form.events,
    active: form.active,
  };
  if (form.secret.trim()) payload.secret = form.secret.trim();
  return payload;
}

/** 목록에 보일 "어디로 가는지". 비밀은 담지 않는다(서버가 준 힌트만). */
export function channelTarget(ch: NotificationChannel): string {
  const s = ch.settings ?? {};
  switch (ch.type) {
    case "telegram":
      return [s.chat_id ? `chat ${s.chat_id}` : "", ch.secret_hint ?? ""]
        .filter(Boolean)
        .join(" · ");
    case "email":
      return `${(s.to ?? []).join(", ")}${s.smtp_host ? ` (${s.smtp_host})` : ""}`;
    default:
      return ch.secret_hint ?? "";
  }
}

export const notificationQueryKeys = {
  channels: () => ["admin", "notifications", "channels"] as const,
  events: () => ["admin", "notifications", "events"] as const,
  history: () => ["admin", "notifications", "history"] as const,
};

const HISTORY_LIMIT = 30;

export const notificationApiCalls = {
  listChannels: () =>
    api
      .get<{
        items: NotificationChannel[];
        total: number;
      }>("/admin/notifications/channels")
      .then((r) => r.data.items ?? []),
  listEvents: () =>
    api
      .get<{
        items: EventSpec[];
        channel_types: ChannelType[];
      }>("/admin/notifications/events")
      .then((r) => r.data.items ?? []),
  listHistory: () =>
    api
      .get<{
        items: NotificationDelivery[];
      }>("/admin/notifications/history", { params: { limit: HISTORY_LIMIT } })
      .then((r) => r.data.items ?? []),
  createChannel: (body: CreateChannelRequest) =>
    api
      .post<NotificationChannel>("/admin/notifications/channels", body)
      .then((r) => r.data),
  updateChannel: (id: string, body: UpdateChannelRequest) =>
    api
      .patch<NotificationChannel>(`/admin/notifications/channels/${id}`, body)
      .then((r) => r.data),
  deleteChannel: (id: string) =>
    api.delete(`/admin/notifications/channels/${id}`).then(() => undefined),
  testChannel: (id: string) =>
    api
      .post<NotificationDelivery>(`/admin/notifications/channels/${id}/test`)
      .then((r) => r.data),
};

export function useNotificationChannels() {
  return useQuery({
    queryKey: notificationQueryKeys.channels(),
    queryFn: notificationApiCalls.listChannels,
  });
}

export function useNotificationEvents() {
  return useQuery({
    queryKey: notificationQueryKeys.events(),
    queryFn: notificationApiCalls.listEvents,
    staleTime: Infinity,
  });
}

export function useNotificationHistory() {
  return useQuery({
    queryKey: notificationQueryKeys.history(),
    queryFn: notificationApiCalls.listHistory,
  });
}

function useInvalidateNotifications() {
  const qc = useQueryClient();
  return () => {
    void qc.invalidateQueries({ queryKey: notificationQueryKeys.channels() });
    void qc.invalidateQueries({ queryKey: notificationQueryKeys.history() });
  };
}

export function useCreateNotificationChannel() {
  const invalidate = useInvalidateNotifications();
  return useMutation({
    mutationFn: notificationApiCalls.createChannel,
    onSuccess: invalidate,
  });
}

export function useUpdateNotificationChannel() {
  const invalidate = useInvalidateNotifications();
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: UpdateChannelRequest }) =>
      notificationApiCalls.updateChannel(id, body),
    onSuccess: invalidate,
  });
}

export function useDeleteNotificationChannel() {
  const invalidate = useInvalidateNotifications();
  return useMutation({
    mutationFn: notificationApiCalls.deleteChannel,
    onSuccess: invalidate,
  });
}

/** 테스트 전송. 결과(sent/failed)가 응답이고 발송 이력에도 남는다 — 이력을 다시 읽는다. */
export function useTestNotificationChannel() {
  const invalidate = useInvalidateNotifications();
  return useMutation({
    mutationFn: notificationApiCalls.testChannel,
    onSuccess: invalidate,
  });
}
