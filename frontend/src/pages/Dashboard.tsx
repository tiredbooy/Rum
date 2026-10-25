import {
  usePauseAllDownloads,
  useStartAllDownloads
} from "@/_lib/services/queries/download.queries";
import { useUndoableDelete } from "@/_lib/services/queries/undo-delete.queries";
import DownloadDialog from "@/features/dashboard/create-download/DownloadDialog";
import { DownloadStatusPage } from "@/features/dashboard/download-list/DownloadStatusPage";
import { FilterTabs } from "@/features/dashboard/FilterTabs";
import { DashboardToolbar } from "@/features/dashboard/Toolbar";
import { BulkActionBar } from "@/features/dashboard/selection/BulkActionBar";
import { useSelection } from "@/features/dashboard/selection/useSelection";
import { useKeyboardShortcuts } from "@/hooks/useKeyboardShortcuts";
import { useAddDialogStore } from "@/stores/add-dialog-store";
import { ListControls } from "@/features/dashboard/list-view/ListControls";
import { VIEW_TABS, type ViewTab } from "@/features/dashboard/list-view/list-view";
import { useListView } from "@/features/dashboard/list-view/useListView";
import { useEffect, useRef } from "react";

export default function Dashboard() {
  const activeTab = useListView((s) => s.tab);
  const setActiveTab = useListView((s) => s.setTab);
  const openWith = useAddDialogStore((s) => s.openWith);
  const filterRef = useRef<HTMLDivElement>(null);

  const startDownloads = useStartAllDownloads();
  const pauseDownloads = usePauseAllDownloads();
  const { removeByStatus } = useUndoableDelete();
  const clearSelection = useSelection((s) => s.clear);

  // Escape clears an active bulk selection (when not typing in a field).
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== "Escape") return;
      const el = document.activeElement;
      const typing =
        el instanceof HTMLInputElement ||
        el instanceof HTMLTextAreaElement ||
        (el instanceof HTMLElement && el.isContentEditable);
      if (!typing) clearSelection();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [clearSelection]);

  const tabs = [
    {
      value: "all",
      label: "All",
      content: <DownloadStatusPage status="all" />,
    },
    {
      value: "running",
      label: "Active",
      content: <DownloadStatusPage status="running" />,
    },
    {
      value: "paused",
      label: "Paused",
      content: <DownloadStatusPage status="paused" />,
    },
    {
      value: "pending",
      label: "Pending",
      content: <DownloadStatusPage status="pending" />,
    },
    {
      value: "completed",
      label: "Completed",
      content: <DownloadStatusPage status="completed" />,
    },
    {
      value: "failed",
      label: "Failed",
      content: <DownloadStatusPage status="error" />,
    },
  ];

  const handleAddDownlad = () => {
    openWith();
  };

  const handleStartAll = () => {
    startDownloads.mutateAsync();
  };

  const handlePauseAll = () => {
    pauseDownloads.mutateAsync();
  };

  const handleDeleteCompleteds = () => {
    void removeByStatus("completed");
  };

  const handleDeleteAll = () => {
    void removeByStatus("all");
    clearSelection();
  };

  const focusFilter = () => {
    // Focus the first filter tab trigger so arrow keys can navigate filters.
    const firstTab = filterRef.current?.querySelector<HTMLElement>(
      "[role='tab']",
    );
    firstTab?.focus();
  };

  useKeyboardShortcuts({
    onNewDownload: () => openWith(),
    onFocusFilter: focusFilter,
    onPauseAll: handlePauseAll,
    onStartAll: handleStartAll,
  });

  return (
    <div className="">
      <DownloadDialog />
      {/* Global drag-&-drop "add" is provided app-wide by <DropProvider /> in
          App.tsx (features/drag-drop) — not re-mounted here to avoid duplicate
          window listeners. */}
      <DashboardToolbar
        stats={{
          active: 1,
          completedToday: 2,
          speed: "2.1",
          dataToday: "12.5",
          failedCount: 12,
        }}
        onAddDownload={handleAddDownlad}
        onPauseAll={handlePauseAll}
        onResumeAll={handleStartAll}
        onRetryFailed={handleStartAll}
        onClearCompleted={handleDeleteCompleteds}
        onDeleteAll={handleDeleteAll}
      />
      <ListControls />
      <div ref={filterRef}>
        <FilterTabs
          tabs={tabs}
          value={activeTab}
          onTabChange={(v) => {
            if (VIEW_TABS.includes(v as ViewTab)) setActiveTab(v as ViewTab);
          }}
          className="mt-4"
        />
      </div>
      <BulkActionBar />
    </div>
  );
}
