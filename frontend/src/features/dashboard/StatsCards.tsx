import { Activity, CheckCircle2, Gauge, HardDrive } from "lucide-react";
import StatCard from "./StatsCard";
import { useDashboardStats } from "@/_lib/services/queries/download.queries";
import { formatDataSize } from "@/_lib/utils/format";

export default function StatsCards() {
  const { data, isLoading, error } = useDashboardStats();

  if (isLoading) {
    return (
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3 pt-4">
        <StatCard
          icon={<Activity className="w-5 h-5" />}
          label="Active"
          value="--"
          suffix="now"
          colorClass="text-cyan-400"
        />
        <StatCard
          icon={<CheckCircle2 className="w-5 h-5" />}
          label="Completed Today"
          value="--"
          colorClass="text-emerald-400"
        />
        <StatCard
          icon={<Gauge className="w-5 h-5" />}
          label="Speed"
          value="--"
          suffix="MB/s"
          colorClass="text-violet-400"
        />
        <StatCard
          icon={<HardDrive className="w-5 h-5" />}
          label="Today's Data"
          value="--"
          suffix="GB"
          colorClass="text-amber-400"
        />
      </div>
    );
  }

  if (error) {
    return (
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3 pt-4">
        <StatCard
          icon={<Activity className="w-5 h-5" />}
          label="Active"
          value="Error"
          suffix="now"
          colorClass="text-red-400"
        />
        <StatCard
          icon={<CheckCircle2 className="w-5 h-5" />}
          label="Completed Today"
          value="Error"
          colorClass="text-emerald-400"
        />
        <StatCard
          icon={<Gauge className="w-5 h-5" />}
          label="Speed"
          value="Error"
          suffix="MB/s"
          colorClass="text-violet-400"
        />
        <StatCard
          icon={<HardDrive className="w-5 h-5" />}
          label="Today's Data"
          value="Error"
          suffix="GB"
          colorClass="text-amber-400"
        />
      </div>
    );
  }

  const active = data?.active_downloads ?? 0;
  const completedToday = data?.completed_today ?? 0;
  const speed = data?.current_speed_mbps?.toFixed(1) ?? "0.0";
  const dataTodayGB = data?.downloaded_today_gb?.toFixed(3) ?? "0.0";
  const { value: dataTodayValue, unit: dataTodayUnit } = formatDataSize(
    Number(dataTodayGB),
  );

  return (
    <div className="grid grid-cols-2 md:grid-cols-4 gap-3 pt-4">
      <StatCard
        icon={<Activity className="w-5 h-5" />}
        label="Active"
        value={active}
        suffix="now"
        colorClass="text-cyan-400"
      />
      <StatCard
        icon={<CheckCircle2 className="w-5 h-5" />}
        label="Completed Today"
        value={completedToday}
        colorClass="text-emerald-400"
      />
      <StatCard
        icon={<Gauge className="w-5 h-5" />}
        label="Speed"
        value={speed}
        suffix="MB/s"
        colorClass="text-violet-400"
      />
      <StatCard
        icon={<HardDrive className="w-5 h-5" />}
        label="Today's Data"
        value={dataTodayValue}
        suffix={dataTodayUnit}
        colorClass="text-amber-400"
      />
    </div>
  );
}
