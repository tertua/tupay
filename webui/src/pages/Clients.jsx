import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Plus } from "lucide-react";
import { PageHeader } from "@/components/layout/PageHeader";
import { Button } from "@/components/ui/Button";
import { EmptyState } from "@/components/ui/EmptyState";
import { ClientsIllo } from "@/components/ui/EmptyIllustrations";
import { Skeleton } from "@/components/ui/Skeleton";
import { QueryError } from "@/components/ui/QueryError";
import { Paginator } from "@/components/ui/Pagination";
import { ClientFormModal } from "@/components/clients/ClientFormModal";
import { ClientCard } from "@/components/clients/ClientCard";
import { ClientsToolbar } from "@/components/clients/ClientsToolbar";
import { useClients } from "@/hooks/useClients";
import { useLang } from "@/context/LangContext";

// Debounce the search box so typing does not fire a request per keystroke.
function useDebounced(value, delay = 300) {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const id = setTimeout(() => setDebounced(value), delay);
    return () => clearTimeout(id);
  }, [value, delay]);
  return debounced;
}

export default function Clients() {
  const nav = useNavigate();
  const { t } = useLang();
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const [sort, setSort] = useState("created_at:desc");
  const [page, setPage] = useState(1);
  const [modalOpen, setModalOpen] = useState(false);

  const query = useDebounced(search);
  const [sortBy, order] = useMemo(() => sort.split(":"), [sort]);

  // Reset to the first page whenever a filter changes.
  useEffect(() => {
    setPage(1);
  }, [query, status, sort]);

  const { data, isLoading, error } = useClients({
    q: query.trim() || undefined,
    status: status || undefined,
    sort: sortBy,
    order,
    page,
  });

  const clients = data?.clients || [];
  const meta = data?.meta;
  const hasFilters = query.trim() !== "" || status !== "";

  return (
    <div>
      <PageHeader
        title={t("clients.title")}
        description={t("clients.desc")}
        actions={
          <Button variant="accent" onClick={() => setModalOpen(true)}>
            <Plus size={16} /> {t("clients.add")}
          </Button>
        }
      />

      <ClientsToolbar
        search={search}
        onSearch={setSearch}
        status={status}
        onStatus={setStatus}
        sort={sort}
        onSort={setSort}
      />

      {isLoading ? (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-[180px] rounded-3xl" />
          ))}
        </div>
      ) : error ? (
        <QueryError error={error} />
      ) : clients.length === 0 ? (
        <EmptyState
          illustration={<ClientsIllo />}
          title={hasFilters ? t("clients.noMatching") : t("clients.noneYet")}
          description={hasFilters ? t("clients.tryDifferent") : t("clients.addFirst")}
          action={
            !hasFilters && (
              <Button variant="accent" onClick={() => setModalOpen(true)}>
                <Plus size={16} /> {t("clients.add")}
              </Button>
            )
          }
        />
      ) : (
        <>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {clients.map((c) => (
              <ClientCard key={c.id} client={c} onOpen={(id) => nav(`/clients/${id}`)} />
            ))}
          </div>
          <Paginator meta={meta} onPage={setPage} />
        </>
      )}

      <ClientFormModal open={modalOpen} onClose={() => setModalOpen(false)} />
    </div>
  );
}
