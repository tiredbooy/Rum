import {
  useDeleteDownloads,
  usePauseAllDownloads,
  usePauseDownload,
  useStartAllDownloads,
} from "@/_lib/services/queries/download.queries";
import { DownloadStatusPage } from "@/features/dashboard/DownloadStatusPage";
import { FilterTabs } from "@/features/dashboard/FilterTabs";
import { DashboardToolbar } from "@/features/dashboard/Toolbar";
import { useState } from "react";

interface Props {
  // props here
}

export default function Dashboard({}: Props) {
  const [activeTab, setActiveTab] = useState("all");

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
      content: <DownloadStatusPage status="failed" />,
    },
  ];

  const handleStartAll = () => {
    startDownloads.mutateAsync();
  };

  const handlePauseAll = () => {
    pauseDownloads.mutateAsync()
  }

  const handleDeleteCompleteds = () => {
    deleteDownloads.mutateAsync("completed");
  };

  return (
    <div className="">
      <DashboardToolbar
        stats={{
          active: 1,
          completedToday: 2,
          speed: "2.1",
          dataToday: "12.5",
          failedCount: 12,
        }}
        onAddDownload={handleStartAll}
        onPauseAll={handlePauseAll}
        onResumeAll={handleStartAll}
        onRetryFailed={() => console.log("value2:")}
        onClearCompleted={handleDeleteCompleteds}
      />
      <FilterTabs
        tabs={tabs}
        defaultValue="all"
        onTabChange={setActiveTab}
        className="mt-4"
      />
    </div>
  );
}
