import { motion } from "framer-motion";
import { Card, CardContent } from "@/components/ui/card";

interface StatCardProps {
  icon: React.ReactNode;
  label: string;
  value: string | number;
  suffix?: string;
  colorClass: string;
}

export default function StatCard({
  icon,
  label,
  value,
  suffix,
  colorClass,
}: StatCardProps) {
  return (
    <motion.div whileHover={{ scale: 1.02 }} whileTap={{ scale: 0.98 }}>
      <Card className="relative bg-card/60 backdrop-blur-md border border-border/40 shadow-sm hover:shadow-md transition-shadow overflow-hidden group">
        {/* Thin accent top line */}
        <div
          className={`absolute top-0 left-0 right-0 h-0.5 bg-linear-to-r ${colorClass.replace("text-", "from-")} to-transparent opacity-50`}
        />

        <CardContent className="p-3 flex items-center gap-3">
          {/* Small icon container */}
          <div
            className={`w-8 h-8 rounded-lg bg-background/80 flex items-center justify-center ${colorClass} shadow-inner group-hover:scale-105 transition-transform shrink-0`}
          >
            {icon}
          </div>

          {/* Value + label */}
          <div className="min-w-0">
            <div className="flex items-baseline gap-1">
              <span className="text-2xl font-bold tracking-tight text-foreground leading-none">
                {value}
              </span>
              {suffix && (
                <span className="text-xs font-medium text-muted-foreground">
                  {suffix}
                </span>
              )}
            </div>
            <p className="text-[10px] font-medium text-muted-foreground uppercase tracking-wider mt-0.5">
              {label}
            </p>
          </div>
        </CardContent>
      </Card>
    </motion.div>
  );
}
