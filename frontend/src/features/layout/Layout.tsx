import { SidebarProvider, SidebarTrigger } from "@/components/ui/sidebar";
import { Outlet } from "react-router-dom";
import { AppSidebar } from "./Sidebar";
import { ConnectionBanner } from "@/components/connection-banner";
import { DiskFullAlert } from "./DiskFullAlert";
import { useProgressStore } from "@/stores/download-progress-store";

export default function Layout() {
  const online = useProgressStore((s) => s.online);
  const networkBlocked = useProgressStore((s) => s.networkBlocked);
  // Two honest states: Rum's own backend is down, or the internet is the problem.
  const message = online
    ? `Network problem. ${networkBlocked} ${networkBlocked === 1 ? "download" : "downloads"} can't continue. Check internet or VPN.`
    : "Rum's engine isn't responding. Retrying…";

  return (
    <SidebarProvider defaultOpen={true}>
      <div className="flex h-screen w-screen overflow-hidden text-foreground">
        <AppSidebar />

        {/* Main panel */}
        <div className="flex flex-1 flex-col overflow-hidden">
          <header className="flex h-16 items-center gap-3 border-b border-border bg-sidebar px-4">
            <SidebarTrigger className="-ml-1" />
            <h2 className="text-lg font-semibold">Rum</h2>
          </header>

          {/* Banner: backend unreachable, or downloads blocked by the network. */}
          <ConnectionBanner online={online && networkBlocked === 0} message={message} />

          {/* Downloads stopped by a full disk (resume by themselves). */}
          <DiskFullAlert />

          <main className="flex-1 overflow-y-auto p-6">
            <Outlet />
          </main>
        </div>
      </div>
    </SidebarProvider>
  );
}
