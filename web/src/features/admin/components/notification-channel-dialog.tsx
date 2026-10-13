import { useId, useMemo, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Modal } from "../../../components/ui/modal";
import { Button } from "../../../components/ui/button";
import { Checkbox } from "../../../components/ui/checkbox";
import { TextInput, textInputClass } from "../../../components/ui/text-input";
import { cn } from "../../../lib/utils";
import {
  CHANNEL_TYPES,
  emptyChannelForm,
  formFromChannel,
  parseRecipients,
  type ChannelForm,
  type ChannelType,
  type EventSpec,
  type NotificationChannel,
  type SMTPSecurity,
} from "../api/notification-api";

const SECURITIES: SMTPSecurity[] = ["starttls", "tls", "none"];

interface Props {
  open: boolean;
  onClose: () => void;
  /** 편집할 채널. 없으면 새로 만든다. */
  channel?: NotificationChannel;
  events: EventSpec[];
  submitting: boolean;
  /** 서버가 돌려준 검증 오류. */
  error?: string;
  onSubmit: (form: ChannelForm) => void;
}

/** 채널을 만들거나 고친다. 채널 종류마다 필요한 칸만 보인다. 비밀은 비밀번호 칸이고, 편집 때는 비어 있다. */
export function NotificationChannelDialog({
  open,
  onClose,
  channel,
  events,
  submitting,
  error,
  onSubmit,
}: Props) {
  const { t } = useTranslation();
  const editing = Boolean(channel);
  const [form, setForm] = useState<ChannelForm>(() =>
    channel
      ? formFromChannel(channel)
      : emptyChannelForm(events.filter((e) => e.default_on).map((e) => e.type)),
  );
  const set = <K extends keyof ChannelForm>(key: K, value: ChannelForm[K]) =>
    setForm((f) => ({ ...f, [key]: value }));

  // 저장된 SMTP 비밀번호는 그 서버로만 간다. 서버 · 포트 · 보안 · 계정을 바꾸면 다시 받는다 — 서버도
  // 같은 규칙으로 거절한다(비밀번호를 모르는 사람이 서버만 돌려 빼내지 못하게).
  const original = useMemo(
    () => (channel ? formFromChannel(channel) : undefined),
    [channel],
  );
  const smtpEndpointChanged =
    original !== undefined &&
    form.type === "email" &&
    (form.smtpHost.trim() !== original.smtpHost ||
      form.smtpPort !== original.smtpPort ||
      form.smtpSecurity !== original.smtpSecurity ||
      form.username.trim() !== original.username);

  // 서버가 최종 판단한다. 여기서는 빈 칸만 막는다 — 형식 오류는 서버 문구가 더 정확하다.
  const ready = useMemo(() => {
    if (!form.name.trim() || form.events.length === 0) return false;
    const secretOk =
      editing && channel?.secret_set && !smtpEndpointChanged
        ? true
        : Boolean(form.secret.trim());
    switch (form.type) {
      case "telegram":
        return secretOk && Boolean(form.chatId.trim());
      case "slack":
        return secretOk;
      case "email": {
        const needsPassword = Boolean(form.username.trim());
        return (
          Boolean(form.smtpHost.trim()) &&
          Boolean(form.from.trim()) &&
          parseRecipients(form.to).length > 0 &&
          (!needsPassword || secretOk)
        );
      }
    }
  }, [form, editing, channel, smtpEndpointChanged]);

  // 저장된 비밀은 돌려받지 않는다. 무엇이 들어 있는지는 서버의 힌트(봇 ID · 웹훅 호스트)로만 보인다.
  const secretKeep =
    editing && channel?.secret_set
      ? channel.secret_hint
        ? t("notificationsPage.form.secretKeep", { hint: channel.secret_hint })
        : t("notificationsPage.form.secretKeepNoHint")
      : undefined;
  const secretPlaceholder = secretKeep;

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={
        editing
          ? t("notificationsPage.dialog.editTitle")
          : t("notificationsPage.dialog.createTitle")
      }
      footer={
        <>
          <Button variant="outline" size="sm" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button
            variant="primary"
            size="sm"
            disabled={!ready}
            loading={submitting}
            onClick={() => onSubmit(form)}
          >
            {t("notificationsPage.form.save")}
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-4">
        <Field label={t("notificationsPage.form.name")}>
          {(id) => (
            <TextInput
              id={id}
              value={form.name}
              maxLength={100}
              onChange={(e) => set("name", e.target.value)}
            />
          )}
        </Field>

        <div>
          <div className="mb-1 text-xs font-medium text-[var(--color-text-secondary)]">
            {t("notificationsPage.form.type")}
          </div>
          <RadioRow
            options={CHANNEL_TYPES.map((type) => ({
              value: type,
              label: t(`notificationsPage.types.${type}`),
            }))}
            value={form.type}
            disabled={editing}
            onChange={(v) => set("type", v as ChannelType)}
          />
        </div>

        {form.type === "telegram" && (
          <>
            <Field
              label={t("notificationsPage.form.botToken")}
              hint={secretKeep ?? t("notificationsPage.form.botTokenHint")}
            >
              {(id) => (
                <TextInput
                  id={id}
                  type="password"
                  autoComplete="off"
                  placeholder={secretPlaceholder}
                  value={form.secret}
                  onChange={(e) => set("secret", e.target.value)}
                />
              )}
            </Field>
            <Field
              label={t("notificationsPage.form.chatId")}
              hint={t("notificationsPage.form.chatIdHint")}
            >
              {(id) => (
                <TextInput
                  id={id}
                  value={form.chatId}
                  onChange={(e) => set("chatId", e.target.value)}
                />
              )}
            </Field>
          </>
        )}

        {form.type === "slack" && (
          <Field
            label={t("notificationsPage.form.webhookUrl")}
            hint={secretKeep ?? t("notificationsPage.form.webhookHint")}
          >
            {(id) => (
              <TextInput
                id={id}
                type="password"
                autoComplete="off"
                placeholder={secretPlaceholder}
                value={form.secret}
                onChange={(e) => set("secret", e.target.value)}
              />
            )}
          </Field>
        )}

        {form.type === "email" && (
          <>
            <div className="grid grid-cols-[1fr_120px] gap-3">
              <Field label={t("notificationsPage.form.smtpHost")}>
                {(id) => (
                  <TextInput
                    id={id}
                    value={form.smtpHost}
                    onChange={(e) => set("smtpHost", e.target.value)}
                  />
                )}
              </Field>
              <Field label={t("notificationsPage.form.smtpPort")}>
                {(id) => (
                  <TextInput
                    id={id}
                    inputMode="numeric"
                    placeholder={String(
                      form.smtpSecurity === "tls"
                        ? 465
                        : form.smtpSecurity === "none"
                          ? 25
                          : 587,
                    )}
                    value={form.smtpPort}
                    onChange={(e) =>
                      set("smtpPort", e.target.value.replace(/[^0-9]/g, ""))
                    }
                  />
                )}
              </Field>
            </div>
            <div>
              <div className="mb-1 text-xs font-medium text-[var(--color-text-secondary)]">
                {t("notificationsPage.form.smtpSecurity")}
              </div>
              <RadioRow
                options={SECURITIES.map((s) => ({
                  value: s,
                  label: t(`notificationsPage.form.security.${s}`),
                }))}
                value={form.smtpSecurity}
                onChange={(v) => set("smtpSecurity", v as SMTPSecurity)}
              />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <Field label={t("notificationsPage.form.username")}>
                {(id) => (
                  <TextInput
                    id={id}
                    autoComplete="off"
                    value={form.username}
                    onChange={(e) => set("username", e.target.value)}
                  />
                )}
              </Field>
              <Field
                label={t("notificationsPage.form.password")}
                hint={
                  smtpEndpointChanged && form.username.trim()
                    ? t("notificationsPage.form.passwordAgain")
                    : secretKeep
                }
              >
                {(id) => (
                  <TextInput
                    id={id}
                    type="password"
                    autoComplete="new-password"
                    placeholder={secretPlaceholder}
                    disabled={!form.username.trim()}
                    value={form.secret}
                    onChange={(e) => set("secret", e.target.value)}
                  />
                )}
              </Field>
            </div>
            <Field label={t("notificationsPage.form.from")}>
              {(id) => (
                <TextInput
                  id={id}
                  placeholder="Nullus <nullus@example.com>"
                  value={form.from}
                  onChange={(e) => set("from", e.target.value)}
                />
              )}
            </Field>
            <Field
              label={t("notificationsPage.form.to")}
              hint={t("notificationsPage.form.toHint")}
            >
              {(id) => (
                <textarea
                  id={id}
                  rows={3}
                  className={textInputClass("h-auto py-1.5")}
                  value={form.to}
                  onChange={(e) => set("to", e.target.value)}
                />
              )}
            </Field>
          </>
        )}

        <fieldset>
          <legend className="mb-1 text-xs font-medium text-[var(--color-text-secondary)]">
            {t("notificationsPage.form.events")}
          </legend>
          <div className="flex flex-col gap-1.5">
            {events.map((spec) => (
              <label
                key={spec.type}
                className="flex items-center gap-2 text-sm"
              >
                <Checkbox
                  checked={form.events.includes(spec.type)}
                  onChange={(e) =>
                    set(
                      "events",
                      e.target.checked
                        ? [...form.events, spec.type]
                        : form.events.filter((x) => x !== spec.type),
                    )
                  }
                />
                {t(`notificationsPage.events.${spec.type}`)}
              </label>
            ))}
          </div>
        </fieldset>

        <label className="flex items-center gap-2 text-sm">
          <Checkbox
            checked={form.active}
            onChange={(e) => set("active", e.target.checked)}
          />
          {t("notificationsPage.form.active")}
        </label>

        {error && (
          <p role="alert" className="text-sm text-[var(--color-error)]">
            {error}
          </p>
        )}
      </div>
    </Modal>
  );
}

function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: (id: string) => ReactNode;
}) {
  const id = useId();
  return (
    <div className="flex flex-col gap-1">
      <label
        htmlFor={id}
        className="text-xs font-medium text-[var(--color-text-secondary)]"
      >
        {label}
      </label>
      {children(id)}
      {hint && <p className="text-xs text-[var(--color-text-muted)]">{hint}</p>}
    </div>
  );
}

function RadioRow({
  options,
  value,
  disabled,
  onChange,
}: {
  options: { value: string; label: string }[];
  value: string;
  disabled?: boolean;
  onChange: (value: string) => void;
}) {
  return (
    <div role="radiogroup" className="flex flex-wrap gap-2">
      {options.map((o) => (
        <button
          key={o.value}
          type="button"
          role="radio"
          aria-checked={value === o.value}
          disabled={disabled && value !== o.value}
          onClick={() => onChange(o.value)}
          className={cn(
            "rounded-[var(--radius-sm)] border px-3 py-1.5 text-sm transition-colors",
            value === o.value
              ? "border-[var(--color-primary)] bg-[color-mix(in_srgb,_var(--color-primary)_12%,_transparent)] text-[var(--color-text-primary)]"
              : "border-[var(--color-border-default)] text-[var(--color-text-secondary)] hover:border-[var(--color-border-hover)]",
            "disabled:cursor-not-allowed disabled:opacity-50",
          )}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}
