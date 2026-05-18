// lib/navigation.ts
import {
  LayoutDashboard,
  Download,
  CheckCircle2,
  XCircle,
  Settings,
  type LucideIcon,
} from "lucide-react";

export interface NavItem {
  title: string;
  to: string;
  icon: LucideIcon;
}

export const mainNavItems: NavItem[] = [
  { title: "Dashboard", to: "/", icon: LayoutDashboard },
  { title: "Active Downloads", to: "/active", icon: Download },
  { title: "Completed", to: "/completed", icon: CheckCircle2 },
  { title: "Failed", to: "/failed", icon: XCircle },
  { title: "Settings", to: "/settings", icon: Settings },
];