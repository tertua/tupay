import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { KeyRound, RotateCw, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { adminGatewayApi } from "@/api/adminGateway";
import { StepUpPrompt } from "@/components/auth/StepUpPrompt";
import { AdminDenseCell, AdminDenseRow, AdminDenseTable } from "@/components/admin/AdminDenseTable";
import { StatusPill } from "@/components/gateway/StatusPill";
import { Button } from "@/components/ui/Button";
import { useLang } from "@/context/LangContext";
import { gatewayProjectsKey, useUpdateGatewayProject } from "@/hooks/useGatewayAdmin";
import { useRotateGatewayKey, useRotateGatewaySecret } from "@/hooks/useGatewayRotate";

export function ProjectsTable({ projects, onCredentials }) {
  const { t } = useLang();
  const queryClient = useQueryClient();
  const rotateKey = useRotateGatewayKey();
  const rotateSecret = useRotateGatewaySecret();
  const update = useUpdateGatewayProject();
  const remove = useMutation({
    mutationFn: (slug) => adminGatewayApi.deleteGatewayProject(slug),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: gatewayProjectsKey }),
  });
  // pending is the rotation waiting on a step-up token: {mutation, slug}.
  const [pending, setPending] = useState(null);
  async function rotate(mutation, slug, token) {
    try {
      onCredentials(await mutation.mutateAsync({ slug, token }));
    } catch (error) {
      if (error.stepUpRequired) {
        setPending({ mutation, slug });
        return;
      }
      if (error.status !== 401) toast.error(error.message || t("gateway.saveFailed"));
    }
  }
  async function toggle(project) {
    try {
      await update.mutateAsync({ slug: project.slug, payload: { is_active: !project.is_active } });
    } catch (error) {
      if (error.status !== 401) toast.error(error.message || t("gateway.saveFailed"));
    }
  }
  async function destroy(project) {
    if (!window.confirm(t("gateway.confirmDelete", { name: project.name }))) return;
    try {
      await remove.mutateAsync(project.slug);
      toast.success(t("gateway.deleted"));
    } catch (error) {
      if (error.status !== 401) toast.error(error.message || t("gateway.saveFailed"));
    }
  }
  return (
    <>
      <AdminDenseTable
        minWidthClassName="min-w-[900px]"
        columns={[
          { key: "project", label: t("gateway.project") },
          { key: "gateway", label: t("gateway.gateway") },
          { key: "webhook", label: t("gateway.webhook") },
          { key: "status", label: t("gateway.status") },
          { key: "actions", label: t("gateway.actions"), align: "right" },
        ]}
      >
        {projects.map((project) => (
          <AdminDenseRow key={project.slug} className="align-top">
            <AdminDenseCell><div className="font-medium text-[var(--ink)]">{project.name}</div><div className="mt-0.5 text-xs text-[var(--ink-muted)]">{project.slug}</div></AdminDenseCell>
            <AdminDenseCell><code className="rounded bg-[var(--surface-2)] px-2 py-1 text-xs">{project.default_gateway || t("gateway.noDefault")}</code></AdminDenseCell>
            <AdminDenseCell className="max-w-[260px] truncate text-xs text-[var(--ink-muted)]" title={project.webhook_url}>{project.webhook_url}</AdminDenseCell>
            <AdminDenseCell><StatusPill active={project.is_active} t={t} /></AdminDenseCell>
            <AdminDenseCell className="text-right">
              <div className="flex flex-wrap justify-end gap-2">
                <Button size="sm" variant="ghost" onClick={() => toggle(project)} disabled={update.isPending}>{project.is_active ? t("gateway.disable") : t("gateway.enable")}</Button>
                <Button size="sm" variant="outline" onClick={() => rotate(rotateKey, project.slug)} disabled={rotateKey.isPending}><KeyRound size={13} />{t("gateway.rotateKey")}</Button>
                <Button size="sm" variant="outline" onClick={() => rotate(rotateSecret, project.slug)} disabled={rotateSecret.isPending}><RotateCw size={13} />{t("gateway.rotateSecret")}</Button>
                <Button size="sm" variant="ghost" className="text-[var(--danger)] hover:bg-[var(--danger)]/10" onClick={() => destroy(project)} disabled={remove.isPending}><Trash2 size={13} />{t("common.delete")}</Button>
              </div>
            </AdminDenseCell>
          </AdminDenseRow>
        ))}
      </AdminDenseTable>

      <StepUpPrompt
        open={Boolean(pending)}
        onClose={() => setPending(null)}
        onConfirmed={(token) => {
          if (!pending) return;
          rotate(pending.mutation, pending.slug, token);
        }}
      />
    </>
  );
}
