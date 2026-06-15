import { DownloadStatusPage } from "@/features/dashboard/download-list/DownloadStatusPage";

interface Props {
  // props here
}

export default function CompletedDownloads({}: Props) {
  return <DownloadStatusPage status="completed" title="Completed Downloads" />;
}
