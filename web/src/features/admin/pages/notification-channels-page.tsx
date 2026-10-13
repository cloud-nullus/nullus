import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Send } from "lucide-react";
import { toast } from "sonner";
import { PageHeader } from "../../../components/layout/page-header";
import { Button } from "../../../components/ui/button";
import { Badge } from "../../../components/ui/badge";
import { iconProps } from "../../../components/ui/icon";
import { ConfirmDialog } from "../../../components/shared/confirm-dialog";
import {
  tableHeadRowClass,
  thClass,
} from "../../../components/shared/table-chrome";
import { NotificationChannelDialog } from "../components/notification-channel-dialog";
import {
  buildCreatePayload,
  buildUpdatePayload,
  channelTarget,
  useCreateNotificationChannel,
  useDeleteNotificationChannel,
  useNotificationChannels,
  useNotificationEvents,
  useNotificationHistory,
  useTestNotificationChannel,
  useUpdateNotificationChannel,
  type ChannelForm,
  type NotificationChannel,
  type NotificationDelivery,
} from "../api/notification-api";

const tone = (color: string) =>
  `bg-[color-mix(in_srgb,_var(${color})_15%,_transparent)] text-[var(${color})]`;

function errorMessage(err: unknown): string {
  if (
    err &&
    typeof err === "object" &&
    "message" in err &&
    typeof err.message === "string"
  )
    return err.message;
  return String(err);
}

/**
 * 알림 채널 (nullus-plan#63). 조직의 Telegram · 이메일 · Slack 채널을 등록하고, 받을 이벤트를 고르고,
 * 테스트로 확인한다. 아래에는 최근 발송 결과가 남는다 — 설정만 저장되고 알림이 안 가는 상태를
 * 화면에서 바로 알아보게 한다.
 */
export function NotificationChannelsPage() {
  const { t } = useTranslation();
  const { data: channels = [] } = useNotificationChannels();
  const { data: events = [] } = useNotificationEvents();
  const { data: history = [] } = useNotificationHistory();
  const createChannel = useCreateNotificationChannel();
  const updateChannel = useUpdateNotificationChannel();
  const deleteChannel = useDeleteNotificationChannel();
  const testChannel = useTestNotificationChannel();

  const [editing, setEditing] = useState<NotificationChannel | null>(null);
  const [creating, setCreating] = useState(false);
  const [formError, setFormError] = useState<string>();
  const [deleting, setDeleting] = useState<NotificationChannel | null>(null);

  const closeDialog = () => {
    setCreating(false);
    setEditing(null);
    setFormError(undefined);
  };

  const submit = (form: ChannelForm) => {
    const handlers = {
      onSuccess: () => {
        toast.success(t("notificationsPage.saved"));
        closeDialog();
      },
      onError: (err: unknown) => setFormError(errorMessage(err)),
    };
    if (editing) {
      updateChannel.mutate(
        { id: editing.id, body: buildUpdatePayload(form) },
        handlers,
      );
    } else {
      createChannel.mutate(buildCreatePayload(form), handlers);
    }
  };

  const sendTest = (ch: NotificationChannel) => {
    testChannel.mutate(ch.id, {
      onSuccess: (result: Pick<NotificationDelivery, "status" | "error">) => {
        if (result.status === "sent") {
          toast.success(t("notificationsPage.testSent", { name: ch.name }));
        } else {
          toast.error(
            t("notificationsPage.testFailed", {
              name: ch.name,
              error: result.error ?? "",
            }),
          );
        }
      },
      onError: (err: unknown) =>
        toast.error(
          t("notificationsPage.testFailed", {
            name: ch.name,
            error: errorMessage(err),
          }),
        ),
    });
  };

  return (
    <div>
      <PageHeader
        breadcrumb={[{ label: t("notificationsPage.breadcrumb") }]}
        icon={<Send {...iconProps("sm")} />}
        title={t("notificationsPage.title")}
        subtitle={t("notificationsPage.description")}
        actions={
          <Button variant="primary" onClick={() => setCreating(true)}>
            {t("notificationsPage.add")}
          </Button>
        }
      />

      <div className="overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)]">
        <table
          className="w-full text-sm"
          aria-label={t("notificationsPage.title")}
        >
          <thead>
            <tr className={tableHeadRowClass}>
              <th className={thClass}>{t("notificationsPage.table.name")}</th>
              <th className={thClass}>{t("notificationsPage.table.type")}</th>
              <th className={thClass}>{t("notificationsPage.table.target")}</th>
              <th className={thClass}>{t("notificationsPage.table.events")}</th>
              <th className={thClass}>{t("notificationsPage.table.status")}</th>
              <th className={thClass}>
                {t("notificationsPage.table.actions")}
              </th>
            </tr>
          </thead>
          <tbody>
            {channels.length === 0 && (
              <tr>
                <td
                  colSpan={6}
                  className="p-6 text-center text-[var(--color-text-secondary)]"
                >
                  {t("notificationsPage.empty")}
                </td>
              </tr>
            )}
            {channels.map((ch) => (
              <tr
                key={ch.id}
                className="border-t border-[var(--color-border-default)]"
              >
                <td className="p-3 font-medium">{ch.name}</td>
                <td className="p-3">
                  {t(`notificationsPage.types.${ch.type}`)}
                </td>
                <td className="p-3 text-[var(--color-text-secondary)]">
                  {channelTarget(ch)}
                  {ch.secret_unreadable && (
                    <div className="mt-1 text-xs text-[var(--color-error)]">
                      {t("notificationsPage.secretUnreadable")}
                    </div>
                  )}
                </td>
                <td className="p-3">
                  <div className="flex flex-wrap gap-1">
                    {ch.events.map((e) => (
                      <Badge key={e} className={tone("--color-text-secondary")}>
                        {t(`notificationsPage.events.${e}`, e)}
                      </Badge>
                    ))}
                  </div>
                </td>
                <td className="p-3">
                  <Badge
                    className={
                      ch.active
                        ? tone("--color-success")
                        : tone("--color-text-secondary")
                    }
                  >
                    {ch.active
                      ? t("notificationsPage.status.active")
                      : t("notificationsPage.status.inactive")}
                  </Badge>
                </td>
                <td className="p-3">
                  <div className="flex gap-2">
                    <Button
                      size="sm"
                      variant="outline"
                      loading={
                        testChannel.isPending && testChannel.variables === ch.id
                      }
                      onClick={() => sendTest(ch)}
                    >
                      {t("notificationsPage.actions.test")}
                    </Button>
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() => setEditing(ch)}
                    >
                      {t("notificationsPage.actions.edit")}
                    </Button>
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() => setDeleting(ch)}
                    >
                      {t("notificationsPage.actions.delete")}
                    </Button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <h2 className="mb-2 mt-8 text-sm font-semibold">
        {t("notificationsPage.history.title")}
      </h2>
      <div className="overflow-hidden rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)]">
        <table
          className="w-full text-sm"
          aria-label={t("notificationsPage.history.title")}
        >
          <thead>
            <tr className={tableHeadRowClass}>
              <th className={thClass}>{t("notificationsPage.history.time")}</th>
              <th className={thClass}>
                {t("notificationsPage.history.channel")}
              </th>
              <th className={thClass}>
                {t("notificationsPage.history.event")}
              </th>
              <th className={thClass}>
                {t("notificationsPage.history.message")}
              </th>
              <th className={thClass}>
                {t("notificationsPage.history.result")}
              </th>
              <th className={thClass}>
                {t("notificationsPage.history.attempts")}
              </th>
            </tr>
          </thead>
          <tbody>
            {history.length === 0 && (
              <tr>
                <td
                  colSpan={6}
                  className="p-6 text-center text-[var(--color-text-secondary)]"
                >
                  {t("notificationsPage.history.empty")}
                </td>
              </tr>
            )}
            {history.map((d) => (
              <tr
                key={d.id}
                className="border-t border-[var(--color-border-default)]"
              >
                <td className="p-3 whitespace-nowrap">
                  {new Date(d.created_at).toLocaleString()}
                </td>
                <td className="p-3">
                  {d.channel_name || "—"}
                  <span className="ml-1 text-xs text-[var(--color-text-muted)]">
                    {d.channel_type
                      ? t(`notificationsPage.types.${d.channel_type}`)
                      : ""}
                  </span>
                </td>
                <td className="p-3">
                  {t(`notificationsPage.events.${d.event}`, d.event)}
                </td>
                <td className="p-3">{d.title}</td>
                <td className="p-3">
                  <Badge
                    className={
                      d.status === "sent"
                        ? tone("--color-success")
                        : tone("--color-error")
                    }
                  >
                    {d.status === "sent"
                      ? t("notificationsPage.result.sent")
                      : t("notificationsPage.result.failed")}
                  </Badge>
                  {d.error && (
                    <div className="mt-1 text-xs text-[var(--color-text-secondary)]">
                      {d.error}
                    </div>
                  )}
                </td>
                <td className="p-3">{d.attempts}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {(creating || editing) && (
        <NotificationChannelDialog
          open
          onClose={closeDialog}
          channel={editing ?? undefined}
          events={events}
          submitting={createChannel.isPending || updateChannel.isPending}
          error={formError}
          onSubmit={submit}
        />
      )}

      <ConfirmDialog
        open={deleting !== null}
        onClose={() => setDeleting(null)}
        onConfirm={() => {
          if (!deleting) return;
          deleteChannel.mutate(deleting.id, {
            onSuccess: () => {
              toast.success(t("notificationsPage.deleted"));
              setDeleting(null);
            },
            onError: (err: unknown) => toast.error(errorMessage(err)),
          });
        }}
        title={t("notificationsPage.confirmDelete.title")}
        description={t("notificationsPage.confirmDelete.description", {
          name: deleting?.name ?? "",
        })}
        confirmLabel={t("notificationsPage.confirmDelete.title")}
        loading={deleteChannel.isPending}
      />
    </div>
  );
}
