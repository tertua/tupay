import { useState } from "react";
import { toast } from "sonner";
import { AdminDenseCell, AdminDenseCellMuted, AdminDenseRow, AdminDenseTable } from "@/components/admin/AdminDenseTable";
import { UserStatusButton } from "@/components/admin/UserStatusButton";
import { UserDeleteButton } from "@/components/admin/UserDeleteButton";
import { useLang } from "@/context/LangContext";
import { useUpdateUserRole } from "@/hooks/useAdminUsers";
import { formatDate } from "@/lib/utils";
import { cn } from "@/lib/utils";

const ROLES = ["user", "admin"];

function userStatus(status, t) {
  return status === 1 ? t("admin.statusActive") : t("admin.statusBlocked");
}

function RoleBadge({ role }) {
  const { t } = useLang();
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full px-2.5 py-1 text-[11px] font-semibold capitalize",
        role === "admin"
          ? "bg-[var(--accent-soft)] text-[var(--accent-strong)]"
          : "bg-[var(--surface-2)] text-[var(--ink-muted)]",
      )}
    >
      {t(`admin.role.${role}`)}
    </span>
  );
}

function UserRow({ account, currentUserId }) {
  const { t } = useLang();
  const updateRole = useUpdateUserRole();
  const [role, setRole] = useState(account.role);
  const isCurrentUser = account.id === currentUserId;

  async function onRoleChange(event) {
    const nextRole = event.target.value;
    setRole(nextRole);
    try {
      await updateRole.mutateAsync({ id: account.id, role: nextRole });
      toast.success(t("admin.roleUpdated"));
    } catch (error) {
      setRole(account.role);
      if (error?.status !== 401) toast.error(t("admin.roleUpdateFailed"), { description: error?.message });
    }
  }

  return (
    <AdminDenseRow>
      <AdminDenseCell>
        <div className="font-medium text-[var(--ink)]">{account.name}</div>
        <div className="mt-0.5 text-xs text-[var(--ink-muted)]">{account.email}</div>
      </AdminDenseCell>
      <AdminDenseCell><RoleBadge role={account.role} /></AdminDenseCell>
      <AdminDenseCellMuted>{userStatus(account.status, t)}</AdminDenseCellMuted>
      <AdminDenseCellMuted>{formatDate(account.created_at)}</AdminDenseCellMuted>
      <AdminDenseCell className="text-right">
        {isCurrentUser ? (
          <span className="text-xs text-[var(--ink-muted)]">{t("admin.you")}</span>
        ) : (
          <div className="flex items-center justify-end gap-2">
            <UserStatusButton account={account} />
            <UserDeleteButton account={account} />
            <select
              aria-label={t("admin.changeRoleFor", { name: account.name })}
              value={role}
              onChange={onRoleChange}
              disabled={updateRole.isPending}
              className="h-9 rounded-full border border-[var(--border)] bg-[var(--surface)] px-3 text-xs font-medium text-[var(--ink)] outline-none focus:border-[var(--accent)]/50 focus:ring-2 focus:ring-[var(--accent)]/15 disabled:opacity-50 disabled:pointer-events-none"
            >
              {ROLES.map((value) => <option key={value} value={value}>{t(`admin.role.${value}`)}</option>)}
            </select>
          </div>
        )}
      </AdminDenseCell>
    </AdminDenseRow>
  );
}

export function AdminUsersTable({ users, currentUserId }) {
  const { t } = useLang();
  return (
    <AdminDenseTable
      minWidthClassName="min-w-[700px]"
      columns={[
        { key: "user", label: t("admin.user") },
        { key: "role", label: t("admin.role") },
        { key: "status", label: t("admin.status") },
        { key: "joined", label: t("admin.joined") },
        { key: "action", label: t("admin.action"), align: "right" },
      ]}
    >
      {users.map((account) => (
        <UserRow key={account.id} account={account} currentUserId={currentUserId} />
      ))}
    </AdminDenseTable>
  );
}
