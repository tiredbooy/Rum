import { SidebarProvider, SidebarTrigger } from "@/components/ui/sidebar";
import { Link, Outlet } from "react-router-dom";
import { AppSidebar } from "./Sidebar";
import { ConnectionBanner } from "@/components/connection-banner";
import { DiskFullAlert } from "./DiskFullAlert";
import { bannerMessage } from "@/_lib/network-banner";
import { useNetworkBanner } from "./useNetworkBanner";

export default function Layout() {
  const { kind, failed } = useNetworkBanner();
  const back = kind === "back";

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

          {/* Banner: engine down, internet offline, or briefly back online. */}
          <ConnectionBanner
            online={kind === "none"}
            resetKey={kind}
            tone={back ? "success" : "warning"}
            message={bannerMessage(kind, failed)}
            action={
              back && failed > 0 ? (
                <Link to="/failed" className="font-medium underline underline-offset-2">
                  Retry failed
                </Link>
              ) : undefined
            }
          />

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
