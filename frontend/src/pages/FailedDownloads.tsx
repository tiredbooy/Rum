import { DownloadStatusPage } from "@/features/dashboard/download-list/DownloadStatusPage";

interface Props {
  // props here
}

export default function FailedDownloads({}: Props) {
  return <DownloadStatusPage status="error" title="Failed Downloads" />;
}
