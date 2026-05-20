import { usePauseDownload, useStartAllDownloads } from "@/_lib/services/queries/download.queries";
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
  const pauseDownloads = usePauseDownload();

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
    startDownloads.mutateAsync()
  }

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
        onPauseAll={() => console.log("value2:")}
        onResumeAll={() => console.log("value2:")}
        onRetryFailed={() => console.log("value2:")}
        onClearCompleted={() => console.log("value2:")}
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
