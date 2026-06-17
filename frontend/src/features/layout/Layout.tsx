import { SidebarProvider, SidebarTrigger } from "@/components/ui/sidebar";
import { Outlet } from "react-router-dom";
import { AppSidebar } from "./Sidebar";
import { ConnectionBanner } from "@/components/connection-banner";
import { useProgressStore } from "@/stores/download-progress-store";

export default function Layout() {
  const online = useProgressStore((s) => s.online);

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

          {/* Live-connection banner: shown when the SSE stream drops. */}
          <ConnectionBanner online={online} />

          <main className="flex-1 overflow-y-auto p-6">
            <Outlet />
          </main>
        </div>
      </div>
    </SidebarProvider>
  );
}
