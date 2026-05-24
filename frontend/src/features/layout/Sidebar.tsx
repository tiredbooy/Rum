import { NavLink, useLocation } from "react-router-dom";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar";
import { mainNavItems } from "@/_lib/config/navigations";
import { PackageOpen } from "lucide-react";
import pkg from "../../../package.json";

const version = pkg.version;

export function AppSidebar() {
  const location = useLocation();
  return (
    <Sidebar collapsible="icon" variant="sidebar">
      {/* ---- Branding ---- */}
      <SidebarHeader className="px-3 pt-4 pb-2 mb-2 border-b border-sidebar-border/40">
        <div className="flex items-center gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-gradient-to-br from-sidebar-primary to-sidebar-primary/70 text-sidebar-primary-foreground shadow-sm">
            <PackageOpen className="h-5 w-5" />
          </div>
          <div className="flex flex-col group-data-[collapsible=icon]:hidden">
            <span className="text-lg font-bold tracking-tight text-sidebar-foreground">
              Rum
            </span>
            <span className="text-[10px] text-sidebar-foreground/50 leading-none mt-0.5">
              Download Manager
            </span>
          </div>
        </div>
      </SidebarHeader>

      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupLabel className="…">Library</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu className="space-y-0.5">
              {mainNavItems
                .filter((item) => item.to !== "/settings")
                .map((item) => {
                  // Reactive isActive – exactly like your original code
                  const isActive =
                    item.to === "/"
                      ? location.pathname === "/"
                      : location.pathname.startsWith(item.to);

                  return (
                    <SidebarMenuItem key={item.to}>
                      {/* Set isActive={false} to prevent the button’s own active style */}
                      <SidebarMenuButton
                        asChild
                        tooltip={item.title}
                        isActive={false}
                      >
                        <NavLink
                          to={item.to}
                          // Plain string – no callback
                          className={`flex items-center gap-3 rounded-md px-2 py-1.5 text-sm font-medium transition-all duration-200 border-l-2 ${
                            isActive
                              ? "border-sidebar-primary bg-sidebar-primary/10 text-sidebar-primary"
                              : "border-transparent text-sidebar-foreground hover:border-sidebar-border hover:bg-sidebar-accent"
                          }`}
                        >
                          <item.icon className="h-4 w-4 shrink-0" />
                          <span>{item.title}</span>
                        </NavLink>
                      </SidebarMenuButton>
                    </SidebarMenuItem>
                  );
                })}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>

        {/* System group – same pattern, just remove the ‘end’ and callback */}
        <SidebarGroup>
          <SidebarGroupLabel className="…">System</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu className="space-y-0.5">
              {mainNavItems
                .filter((item) => item.to === "/settings")
                .map((item) => {
                  const isActive = location.pathname.startsWith(item.to);
                  return (
                    <SidebarMenuItem key={item.to}>
                      <SidebarMenuButton
                        asChild
                        tooltip={item.title}
                        isActive={false}
                      >
                        <NavLink
                          to={item.to}
                          className={`flex items-center gap-3 rounded-md px-2 py-1.5 text-sm font-medium transition-all duration-200 border-l-2 ${
                            isActive
                              ? "border-sidebar-primary bg-sidebar-primary/10 text-sidebar-primary"
                              : "border-transparent text-sidebar-foreground hover:border-sidebar-border hover:bg-sidebar-accent"
                          }`}
                        >
                          <item.icon className="h-4 w-4 shrink-0" />
                          <span>{item.title}</span>
                        </NavLink>
                      </SidebarMenuButton>
                    </SidebarMenuItem>
                  );
                })}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>

      {/* ---- Footer ---- */}
      <SidebarFooter className="px-3 py-2">
        <p className="text-[10px] text-sidebar-foreground/50">
          v{version} · Rum
        </p>
      </SidebarFooter>
    </Sidebar>
  );
}
