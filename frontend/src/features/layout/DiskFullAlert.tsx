import { useDownloads } from "@/_lib/services/queries/download.queries";
import { countDiskFull } from "@/_lib/disk-full";
import { DiskFullBanner } from "@/components/disk-full-banner";

/** Wires the disk-full banner to the live downloads list. */
export function DiskFullAlert() {
  const { data } = useDownloads("all");
  return <DiskFullBanner count={countDiskFull(data ?? [])} />;
}

export default DiskFullAlert;
