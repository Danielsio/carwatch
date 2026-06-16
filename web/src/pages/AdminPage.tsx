import { useState } from "react";
import {
  AlertCircle,
  Car,
  Database,
  FileSearch,
  RefreshCw,
  RotateCw,
  ScrollText,
  Shield,
  Users,
} from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import { useAdminStats } from "@/hooks/useAdmin";
import { EmptyState, Skeleton } from "@/components/ui";
import { Button } from "@/components/ui/button";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import {
  StatCard,
  OverviewTab,
  ListingsTab,
  SearchesTab,
  UsersTab,
  CyclesTab,
  LogsTab,
} from "@/components/admin";

type TabKey = "overview" | "cycles" | "logs" | "listings" | "searches" | "users";

const TABS: { key: TabKey; label: string; icon: typeof Car }[] = [
  { key: "overview", label: "סקירה כללית", icon: Database },
  { key: "cycles", label: "סריקות", icon: RotateCw },
  { key: "logs", label: "לוגים", icon: ScrollText },
  { key: "listings", label: "מודעות", icon: Car },
  { key: "searches", label: "חיפושים", icon: FileSearch },
  { key: "users", label: "משתמשים", icon: Users },
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
            <Button
              variant="outline"
              size="icon"
              onClick={() => void refetch()}
              title="רענן נתונים"
            >
              <RefreshCw className="h-3.5 w-3.5" />
            </Button>
          </div>
        }
      />

      <div className="grid gap-4 grid-cols-2 md:grid-cols-4">
        <StatCard
          label="מודעות"
          value={listingsCount}
          icon={Car}
          color="bg-primary/10 text-primary"
        />
        <StatCard
          label="חיפושים"
          value={searchesCount}
          icon={FileSearch}
          color="bg-chart-2/10 text-chart-2"
        />
        <StatCard
          label="משתמשים"
          value={usersCount}
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

      <Tabs
        value={activeTab}
        onValueChange={(v) => setActiveTab(v as TabKey)}
      >
        <TabsList className="w-full justify-start overflow-x-auto scrollbar-hide">
          {TABS.map((t) => (
            <TabsTrigger key={t.key} value={t.key} className="gap-2">
              <t.icon className="h-4 w-4" />
              {t.label}
            </TabsTrigger>
          ))}
        </TabsList>

        <AnimatePresence mode="popLayout">
          <motion.div
            key={activeTab}
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.15 }}
            className="mt-6"
          >
            <TabsContent value="overview" className="mt-0">
              <OverviewTab data={data} onRefresh={() => void refetch()} />
            </TabsContent>
            <TabsContent value="cycles" className="mt-0">
              <CyclesTab />
            </TabsContent>
            <TabsContent value="logs" className="mt-0">
              <LogsTab active={activeTab === "logs"} />
            </TabsContent>
            <TabsContent value="listings" className="mt-0">
              <ListingsTab
                searchId={listingsSearchId}
                onClearFilter={() => setListingsSearchId(null)}
              />
            </TabsContent>
            <TabsContent value="searches" className="mt-0">
              <SearchesTab onViewListings={viewListingsForSearch} />
            </TabsContent>
            <TabsContent value="users" className="mt-0">
              <UsersTab />
            </TabsContent>
          </motion.div>
        </AnimatePresence>
      </Tabs>
    </div>
  );
}
