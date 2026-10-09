import { toast } from "sonner";
import { useLang } from "@/context/LangContext";
import { useDeleteUser } from "@/hooks/useAdminUserDelete";
import { cn } from "@/lib/utils";

// Delete button for one admin user row. The server only deletes accounts
// without invoice records; a 409 carries the invoice count in the error
// description. Hidden for the current admin (self-delete is refused too).
export function UserDeleteButton({ account }) {
  const { t } = useLang();
  const deleteUser = useDeleteUser();

  async function handleDelete() {
    if (!window.confirm(t("admin.confirmDeleteUser", { name: account.name }))) return;
    try {
      await deleteUser.mutateAsync(account.id);
      toast.success(t("admin.userDeleted"));
    } catch (error) {
      if (error?.status !== 401) {
        toast.error(t("admin.deleteFailed"), { description: error?.message });
      }
    }
  }

  return (
    <button
      type="button"
      onClick={handleDelete}
      disabled={deleteUser.isPending}
      title={t("admin.deleteUserTitle", { name: account.name })}
      className={cn(
        "h-9 rounded-full border px-3 text-xs font-medium outline-none transition-colors focus:ring-2 focus:ring-[var(--danger)]/15 disabled:opacity-50 disabled:pointer-events-none",
        "border-[var(--border)] text-[var(--danger)] hover:bg-[var(--danger)]/10",
      )}
    >
      {t("common.delete")}
    </button>
  );
}
