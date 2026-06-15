import { SidebarProvider, SidebarTrigger } from "@/components/ui/sidebar";
import { Outlet } from "react-router-dom";
import { AppSidebar } from "./Sidebar";

export default function Layout() {
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

          <main className="flex-1 overflow-y-auto p-6">
            <Outlet />
          </main>
        </div>
      </div>
    </SidebarProvider>
  );
}
