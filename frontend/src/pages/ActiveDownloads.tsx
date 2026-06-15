import { DownloadStatusPage } from "@/features/dashboard/download-list/DownloadStatusPage";

interface Props {
  // props here
}

export default function ActiveDownloads ({  }: Props) {
  return (
    <DownloadStatusPage status="running" title="Active Downloads" />
  );
};
