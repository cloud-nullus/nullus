import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../../../lib/api";

/**
 * 백업 대상. 서버의 domain.Component 와 값이 같아야 한다.
 *
 * 모르는 값을 보내면 서버가 BACKUP_INVALID_COMPONENT 로 거절한다 — 조용히
 * 버리면 고른 것과 백업된 것이 달라지고, 그 사실은 복구할 때에야 드러난다.
 */
export type BackupComponent =
  | "platform_db"
  | "keycloak_db"
  | "openbao_kv"
  | "ns_resources"
  | "volume";

export type BackupMode = "full" | "platform_only" | "stack_only";
export type BackupStatus =
  | "pending"
  | "running"
  | "succeeded"
  | "partial"
  | "failed";

/**
 * 정지 창을 만드는 것은 볼륨뿐이다.
 *
 * 서버도 같은 규칙으로 확인 문구를 요구한다(handler.requiresQuiesce).
 * 화면이 먼저 알려 주지 않으면 사용자는 "시작" 을 누른 뒤에야 서비스가
 * 멈춘다는 사실을 알게 된다.
 */
export function requiresQuiesce(scope: BackupComponent[]): boolean {
  return scope.includes("volume");
}

export interface BackupRun {
  id: string;
  stack_id?: string;
  mode: BackupMode;
  trigger: string;
  status: BackupStatus;
  total_bytes: number;
  error?: string;
  quiesce_seconds?: number;
  started_at?: string;
  finished_at?: string;
  created_at: string;
}

export interface CreateBackupRequest {
  stack_id?: string;
  namespace?: string;
  mode: BackupMode;
  scope?: BackupComponent[];
  confirm?: string;
}

export const backupQueryKeys = {
  runs: () => ["admin", "backups"] as const,
};

export const backupApiCalls = {
  listRuns: () =>
    api
      .get<{ items: BackupRun[] }>("/admin/backups")
      .then((r) => r.data.items ?? []),
  createBackup: (body: CreateBackupRequest) =>
    api.post<BackupRun>("/admin/backups", body).then((r) => r.data),
};

/**
 * 실행 이력. 진행 중인 백업이 있으면 주기적으로 다시 읽는다.
 *
 * 백업은 볼륨을 포함하면 수 분 걸린다. 사용자가 새로고침해야 결과를 아는
 * 화면은 "끝났는지" 를 묻게 만든다 — 진행 중일 때만 폴링해서, 끝난 뒤에는
 * 불필요한 요청을 남기지 않는다.
 */
export function useBackupRuns() {
  return useQuery({
    queryKey: backupQueryKeys.runs(),
    queryFn: backupApiCalls.listRuns,
    initialData: [] as BackupRun[],
    refetchInterval: (query) => {
      const runs = query.state.data as BackupRun[] | undefined;
      const busy = runs?.some(
        (r) => r.status === "pending" || r.status === "running",
      );
      return busy ? 5000 : false;
    },
  });
}

export function useCreateBackup() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: backupApiCalls.createBackup,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: backupQueryKeys.runs() });
    },
  });
}
