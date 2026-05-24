import { useState } from "react";
import {
  AlertCircle,
  Car,
  Database,
  FileSearch,
  History,
  RefreshCw,
  RotateCw,
  Shield,
  Users,
} from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import { useAdminStats } from "@/hooks/useAdmin";
import { EmptyState, Skeleton } from "@/components/ui";
import { cn } from "@/lib/utils";
import {
  StatCard,
  OverviewTab,
  ListingsTab,
  SearchesTab,
  UsersTab,
  PriceHistoryTab,
  CyclesTab,
} from "@/components/admin";

type TabKey = "overview" | "cycles" | "listings" | "searches" | "users" | "priceHistory";

const TABS: { key: TabKey; label: string; icon: typeof Car }[] = [
  { key: "overview", label: "סקירה כללית", icon: Database },
  { key: "cycles", label: "סריקות", icon: RotateCw },
  { key: "listings", label: "מודעות", icon: Car },
  { key: "searches", label: "חיפושים", icon: FileSearch },
  { key: "users", label: "משתמשים", icon: Users },
  { key: "priceHistory", label: "היסטוריית מחירים", icon: History },
];

function AdminHeader({ action }: { action?: React.ReactNode }) {
  return (
    <motion.div
      initial={{ opacity: 0, y: -8 }}
      animate={{ opacity: 1, y: 0 }}
      className="flex items-center justify-between border-b border-border/50 pb-6"
    >
      <div className="flex items-center gap-3">
        <div className="flex h-11 w-11 items-center justify-center rounded-xl bg-destructive/10">
          <Shield className="h-5 w-5 text-destructive" />
        </div>
        <div>
          <h1 className="text-2xl font-bold text-foreground">ניהול מערכת</h1>
          <p className="text-sm text-muted-foreground mt-0.5">
            גישת מנהל · צפה וערוך את כל הנתונים
          </p>
        </div>
      </div>
      {action}
    </motion.div>
  );
}

export function AdminPage() {
  const { data, isLoading, isError, dataUpdatedAt, refetch } = useAdminStats();
  const [activeTab, setActiveTab] = useState<TabKey>("overview");
  const [listingsSearchId, setListingsSearchId] = useState<number | null>(null);

  function viewListingsForSearch(searchId: number) {
    setListingsSearchId(searchId);
    setActiveTab("listings");
  }

  if (isLoading) {
    return (
      <div className="space-y-6 pb-20 md:pb-4">
        <AdminHeader />
        <div className="grid gap-4 grid-cols-2 md:grid-cols-4">
          {[1, 2, 3, 4].map((i) => (
            <Skeleton key={i} className="h-28 rounded-2xl" />
          ))}
        </div>
        <Skeleton className="h-10 w-80 rounded-xl" />
        <div className="grid gap-4 sm:grid-cols-2">
          {[1, 2].map((i) => (
            <Skeleton key={i} className="h-52 rounded-2xl" />
          ))}
        </div>
      </div>
    );
  }

  if (isError || !data) {
    return (
      <div className="space-y-6 pb-20 md:pb-4">
        <AdminHeader />
        <EmptyState
          icon={AlertCircle}
          title="שגיאה בטעינת הנתונים"
          description="ייתכן שאין לך הרשאות מנהל. ודא שה-admin_email מוגדר בקובץ ההגדרות."
        />
      </div>
    );
  }

  const listingsCount = data.tables["listing_history"] ?? 0;
  const searchesCount = data.tables["searches"] ?? 0;
  const usersCount = data.tables["users"] ?? 0;

  const lastUpdated = new Date(dataUpdatedAt);

  return (
    <div className="space-y-6 pb-20 md:pb-4">
      <AdminHeader
        action={
          <div className="flex items-center gap-2">
            <span className="text-xs text-muted-foreground tabular-nums">
              עדכון: {lastUpdated.toLocaleTimeString("he-IL")}
            </span>
            <button
              type="button"
              onClick={() => void refetch()}
              className="flex h-8 w-8 items-center justify-center rounded-lg bg-secondary text-muted-foreground hover:text-foreground transition-colors"
              title="רענן נתונים"
            >
              <RefreshCw className="h-3.5 w-3.5" />
            </button>
          </div>
        }
      />

      {/* Stat Cards */}
      <div className="grid gap-4 grid-cols-2 md:grid-cols-4">
        <StatCard
          label="מודעות"
          value={listingsCount.toLocaleString("he-IL")}
          icon={Car}
          color="bg-primary/10 text-primary"
        />
        <StatCard
          label="חיפושים"
          value={searchesCount.toLocaleString("he-IL")}
          icon={FileSearch}
          color="bg-chart-2/10 text-chart-2"
        />
        <StatCard
          label="משתמשים"
          value={usersCount.toLocaleString("he-IL")}
          icon={Users}
          color="bg-chart-4/10 text-chart-4"
        />
        <StatCard
          label="אחסון"
          value={data.db.file_size_human}
          icon={Database}
          color="bg-warning/10 text-warning"
          subtitle={data.runtime.uptime + " פעיל"}
        />
      </div>

      {/* Tabs */}
      <div className="flex gap-1 rounded-xl bg-secondary/50 p-1 w-fit flex-wrap">
        {TABS.map((t) => (
          <button
            key={t.key}
            type="button"
            onClick={() => setActiveTab(t.key)}
            className={cn(
              "flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-medium transition-all",
              activeTab === t.key
                ? "bg-card text-foreground shadow-sm border border-border"
                : "text-muted-foreground hover:text-foreground",
            )}
          >
            <t.icon className="h-4 w-4" />
            {t.label}
          </button>
        ))}
      </div>

      {/* Tab Content */}
      <AnimatePresence mode="popLayout">
        <motion.div
          key={activeTab}
          initial={{ opacity: 0, y: 8 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.15 }}
        >
          {activeTab === "overview" && (
            <OverviewTab data={data} onRefresh={() => void refetch()} />
          )}
          {activeTab === "cycles" && <CyclesTab />}
          {activeTab === "listings" && (
            <ListingsTab
              searchId={listingsSearchId}
              onClearFilter={() => setListingsSearchId(null)}
            />
          )}
          {activeTab === "searches" && (
            <SearchesTab onViewListings={viewListingsForSearch} />
          )}
          {activeTab === "users" && <UsersTab />}
          {activeTab === "priceHistory" && <PriceHistoryTab />}
        </motion.div>
      </AnimatePresence>
    </div>
  );
}
