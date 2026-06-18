import {
  useDeleteDownloads,
  usePauseAllDownloads,
  useStartAllDownloads
} from "@/_lib/services/queries/download.queries";
import DownloadDialog from "@/features/dashboard/create-download/DownloadDialog";
import { DownloadStatusPage } from "@/features/dashboard/download-list/DownloadStatusPage";
import { FilterTabs } from "@/features/dashboard/FilterTabs";
import { DashboardToolbar } from "@/features/dashboard/Toolbar";
import { useKeyboardShortcuts } from "@/hooks/useKeyboardShortcuts";
import { useAddDialogStore } from "@/stores/add-dialog-store";
import { useRef, useState } from "react";

export default function Dashboard() {
  const [activeTab, setActiveTab] = useState("all");
  const openWith = useAddDialogStore((s) => s.openWith);
  const filterRef = useRef<HTMLDivElement>(null);

  const startDownloads = useStartAllDownloads();
  const pauseDownloads = usePauseAllDownloads();
  const deleteDownloads = useDeleteDownloads();

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
    deleteDownloads.mutateAsync("completed");
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
      />
      <div ref={filterRef}>
        <FilterTabs
          tabs={tabs}
          defaultValue="all"
          onTabChange={setActiveTab}
          className="mt-4"
        />
      </div>
    </div>
  );
}
