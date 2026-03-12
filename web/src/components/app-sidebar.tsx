import { NavLink } from "react-router-dom";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";
import { saveLanguage } from "@/i18n";
import {
  LayoutDashboard,
  MessageSquare,
  GitBranch,
  BarChart3,
  Bot,
  FolderOpen,
  FileStack,
  ChevronsUpDown,
  Sparkles,
  Shield,
  Coins,
  LogOut,
  Globe,
} from "lucide-react";
import { useWorkbench } from "@/contexts/WorkbenchContext";

const navItems = [
  { to: "/", icon: LayoutDashboard, labelKey: "nav.dashboard" },
  { to: "/chat", icon: MessageSquare, labelKey: "nav.chat" },
  { to: "/issues", icon: GitBranch, labelKey: "nav.flows" },
  { to: "/analytics", icon: BarChart3, labelKey: "nav.analytics" },
  { to: "/usage", icon: Coins, labelKey: "nav.usage" },
  { to: "/sandbox", icon: Shield, labelKey: "nav.sandbox" },
  { to: "/templates", icon: FileStack, labelKey: "nav.templates" },
  { to: "/agents", icon: Bot, labelKey: "nav.agents" },
  { to: "/skills", icon: Sparkles, labelKey: "nav.skills" },
  { to: "/projects", icon: FolderOpen, labelKey: "nav.projects" },
];

export function AppSidebar() {
  const { t, i18n } = useTranslation();
  const { projects, selectedProjectId, setSelectedProjectId, logout } = useWorkbench();
  const [showPicker, setShowPicker] = useState(false);
  const currentProject = useMemo(
    () => projects.find((project) => project.id === selectedProjectId) ?? projects[0] ?? null,
    [projects, selectedProjectId],
  );

  return (
    <aside className="flex h-screen w-56 flex-col border-r bg-sidebar">
      {/* Logo */}
      <div className="flex h-14 items-center gap-2.5 border-b px-5">
        <div className="flex h-7 w-7 items-center justify-center rounded-md bg-primary text-primary-foreground">
          <GitBranch className="h-4 w-4" />
        </div>
        <span className="text-sm font-semibold tracking-tight">AI Workflow</span>
      </div>

      {/* Project switcher */}
      <div className="px-3 pt-3">
        <div className="relative">
          <button
            onClick={() => setShowPicker(!showPicker)}
            className="flex w-full items-center gap-2 rounded-md border bg-background px-2.5 py-2 text-sm transition-colors hover:bg-muted"
          >
            <div className="flex h-6 w-6 shrink-0 items-center justify-center rounded bg-indigo-50">
              <GitBranch className="h-3.5 w-3.5 text-indigo-500" />
            </div>
            <span className="flex-1 truncate text-left text-[13px] font-medium">
              {currentProject?.name ?? t("nav.noProject")}
            </span>
            <ChevronsUpDown className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
          </button>

          {showPicker && projects.length > 0 ? (
            <div className="absolute left-0 right-0 top-full z-50 mt-1 rounded-md border bg-popover p-1 shadow-md">
              {projects.map((project) => (
                <button
                  key={project.id}
                  onClick={() => {
                    setSelectedProjectId(project.id);
                    setShowPicker(false);
                  }}
                  className={cn(
                    "flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-sm transition-colors hover:bg-accent",
                    project.id === currentProject?.id && "bg-accent",
                  )}
                >
                  <GitBranch className="h-3.5 w-3.5 text-muted-foreground" />
                  <span className="truncate">{project.name}</span>
                </button>
              ))}
            </div>
          ) : null}
        </div>
      </div>

      {/* Navigation */}
      <nav className="flex-1 space-y-1 px-3 py-3">
        <div className="mb-2 px-3 text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
          {t("nav.navigation")}
        </div>
        {navItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.to === "/"}
            className={({ isActive }) =>
              cn(
                "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                isActive
                  ? "bg-accent text-accent-foreground"
                  : "text-muted-foreground hover:bg-accent hover:text-accent-foreground",
              )
            }
          >
            <item.icon className="h-4 w-4" />
            {t(item.labelKey)}
          </NavLink>
        ))}
      </nav>

      {/* Language switcher + Logout */}
      <div className="border-t px-3 py-3 space-y-1">
        <button
          onClick={() => {
            const next = i18n.language === "zh-CN" ? "en" : "zh-CN";
            void i18n.changeLanguage(next);
            saveLanguage(next);
          }}
          className="flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm font-medium text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
        >
          <Globe className="h-4 w-4" />
          {i18n.language === "zh-CN" ? "English" : "中文"}
        </button>
        <button
          onClick={logout}
          className="flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm font-medium text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
        >
          <LogOut className="h-4 w-4" />
          {t("nav.logout")}
        </button>
      </div>
    </aside>
  );
}
