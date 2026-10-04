import { useState } from "react";
import { FolderKanban, Send, ShieldCheck, XCircle } from "lucide-react";
import { PageHeader } from "@/components/layout/PageHeader";
import { AdminGatewayCreateForm } from "@/components/admin/AdminGatewayCreateForm";
import { AdminGatewayCredentials } from "@/components/admin/AdminGatewayCredentials";
import { AdminTableSkeleton } from "@/components/admin/AdminTableSkeleton";
import { PageStat, PageStatStrip } from "@/components/admin/PageStatStrip";
import { EmptyState } from "@/components/ui/EmptyState";
import { QueryError } from "@/components/ui/QueryError";
import { GatewayDeliveries } from "@/components/gateway/GatewayDeliveries";
import { ProjectsTable } from "@/components/gateway/ProjectsTable";
import { useLang } from "@/context/LangContext";
import { useGatewayDeliveries, useGatewayProjects } from "@/hooks/useGatewayAdmin";

export default function AdminGateway() {
  const { t } = useLang();
  const { data: projects = [], isLoading, error } = useGatewayProjects();
  const { data: deliveries = [] } = useGatewayDeliveries();
  const [credentials, setCredentials] = useState(null);
  if (isLoading) return <AdminTableSkeleton columns={5} minWidthClassName="min-w-[900px]" />;
  if (error) return <QueryError error={error} />;
  const active = projects.filter((project) => project.is_active).length;
  const failed = deliveries.filter((item) => item.status === "failed").length;
  return (
    <div>
      <PageHeader title={t("gateway.title")} description={t("gateway.desc")} />
      {credentials && <AdminGatewayCredentials project={credentials} onClose={() => setCredentials(null)} />}
      <PageStatStrip>
        <PageStat label={t("admin.statProjects")} value={projects.length} icon={FolderKanban} tone="accent" />
        <PageStat label={t("admin.statActive")} value={active} icon={ShieldCheck} tone="success" />
        <PageStat label={t("admin.statDeliveries")} value={deliveries.length} icon={Send} tone="neutral" />
        <PageStat label={t("admin.statFailed")} value={failed} icon={XCircle} tone={failed ? "danger" : "neutral"} />
      </PageStatStrip>
      <AdminGatewayCreateForm onCreated={setCredentials} />
      {projects.length ? <ProjectsTable projects={projects} onCredentials={setCredentials} /> : <EmptyState icon={ShieldCheck} title={t("gateway.empty")} description={t("gateway.emptyDesc")} />}
      <GatewayDeliveries />
    </div>
  );
}
