import { useEffect, useState } from "react"
import { Routes, Route, Navigate, NavLink, useLocation } from "react-router-dom"
import {
  LayoutDashboard, Network, ShieldCheck, Terminal as TerminalIcon,
  TrendingUp, Bot, CheckCircle2, FileText, Zap, Users, DollarSign, Award,
  Search, Moon, Sun, Laptop, Activity,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { useTheme } from "@/components/ThemeProvider"
import CommandPalette from "@/components/CommandPalette"

import Overview from "@/views/Overview"
import Topology from "@/views/Topology"
import Audit from "@/views/Audit"
import Terminal from "@/views/Terminal"
import Forecast from "@/views/Forecast"
import Agent from "@/views/Agent"
import Verify from "@/views/Verify"
import Plan from "@/views/Plan"
import Causes from "@/views/Causes"
import Fleet from "@/views/Fleet"
import Economics from "@/views/Economics"
import Certificates from "@/views/Certificates"

type NavItem = { path: string; label: string; Icon: typeof LayoutDashboard }
type NavGroup = { label: string; items: NavItem[] }

const GROUPS: NavGroup[] = [
  {
    label: "MONITOR",
    items: [
      { path: "/overview", label: "Overview", Icon: LayoutDashboard },
      { path: "/topology", label: "Topology", Icon: Network },
      { path: "/audit", label: "Audit", Icon: ShieldCheck },
      { path: "/terminal", label: "Terminal", Icon: TerminalIcon },
    ],
  },
  {
    label: "REASON",
    items: [
      { path: "/forecast", label: "Forecast", Icon: TrendingUp },
      { path: "/agent", label: "Agent", Icon: Bot },
      { path: "/verify", label: "Verify", Icon: CheckCircle2 },
      { path: "/plan", label: "Plan", Icon: FileText },
      { path: "/causes", label: "Causes", Icon: Zap },
    ],
  },
  {
    label: "SCALE",
    items: [
      { path: "/fleet", label: "Fleet", Icon: Users },
      { path: "/economics", label: "Economics", Icon: DollarSign },
      { path: "/certificates", label: "Certificates", Icon: Award },
    ],
  },
]

function Sidebar() {
  return (
    <aside className="w-60 shrink-0 border-r bg-card flex flex-col">
      <div className="px-4 py-5 flex items-center gap-2 border-b">
        <div className="h-7 w-7 rounded-md bg-primary flex items-center justify-center">
          <Activity className="h-4 w-4 text-primary-foreground" strokeWidth={2.25} />
        </div>
        <span className="font-semibold tracking-tight text-[15px]">Immortal</span>
      </div>

      <nav className="flex-1 overflow-y-auto py-3">
        {GROUPS.map((group) => (
          <div key={group.label} className="px-3 py-2">
            <div className="px-2 text-[11px] font-semibold tracking-wider text-muted-foreground mb-1">
              {group.label}
            </div>
            <div className="space-y-0.5">
              {group.items.map(({ path, label, Icon }) => (
                <NavLink
                  key={path}
                  to={path}
                  className={({ isActive }) =>
                    `flex items-center gap-2.5 px-2 py-1.5 rounded-md text-sm font-medium transition-colors ${
                      isActive
                        ? "bg-secondary text-foreground"
                        : "text-muted-foreground hover:bg-secondary/60 hover:text-foreground"
                    }`
                  }
                >
                  <Icon className="h-4 w-4" strokeWidth={1.8} />
                  <span>{label}</span>
                </NavLink>
              ))}
            </div>
          </div>
        ))}
      </nav>

      <div className="p-3 border-t">
        <div className="px-2 py-1.5 flex items-center gap-2 text-xs text-muted-foreground">
          <span className="h-1.5 w-1.5 rounded-full bg-emerald-500 animate-pulse"></span>
          <span className="font-mono">demo-node</span>
        </div>
      </div>
    </aside>
  )
}

function TopBar({ onOpenPalette }: { onOpenPalette: () => void }) {
  const loc = useLocation()
  const seg = loc.pathname.replace(/^\//, "") || "overview"
  const crumb = seg.charAt(0).toUpperCase() + seg.slice(1)
  const group =
    GROUPS.find((g) => g.items.some((i) => i.path === loc.pathname))?.label.toLowerCase() ?? "monitor"
  const groupName = group.charAt(0).toUpperCase() + group.slice(1)

  const { theme, setTheme } = useTheme()

  return (
    <header className="h-14 border-b bg-card flex items-center px-6 gap-4">
      <div className="text-sm text-muted-foreground">
        <span>{groupName}</span>
        <span className="mx-2 text-muted-foreground/50">/</span>
        <span className="text-foreground font-medium">{crumb}</span>
      </div>

      <Button
        variant="outline"
        size="sm"
        className="ml-auto w-72 justify-between font-normal text-muted-foreground"
        onClick={onOpenPalette}
      >
        <div className="flex items-center gap-2">
          <Search className="h-3.5 w-3.5" />
          <span>Search commands...</span>
        </div>
        <kbd className="pointer-events-none inline-flex h-5 select-none items-center gap-1 rounded border bg-muted px-1.5 font-mono text-[10px] font-medium text-muted-foreground">
          <span className="text-xs">⌘</span>K
        </kbd>
      </Button>

      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" size="icon">
            {theme === "dark" ? <Moon className="h-4 w-4" /> : theme === "light" ? <Sun className="h-4 w-4" /> : <Laptop className="h-4 w-4" />}
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem onClick={() => setTheme("light")}>
            <Sun className="h-4 w-4 mr-2" /> Light
          </DropdownMenuItem>
          <DropdownMenuItem onClick={() => setTheme("dark")}>
            <Moon className="h-4 w-4 mr-2" /> Dark
          </DropdownMenuItem>
          <DropdownMenuItem onClick={() => setTheme("system")}>
            <Laptop className="h-4 w-4 mr-2" /> System
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </header>
  )
}

export default function App() {
  const [paletteOpen, setPaletteOpen] = useState(false)

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const isMod = e.metaKey || e.ctrlKey
      if ((isMod && e.key.toLowerCase() === "k") || (e.key === "/" && !(e.target as HTMLElement).closest("input, textarea"))) {
        e.preventDefault()
        setPaletteOpen((o) => !o)
      }
    }
    document.addEventListener("keydown", onKey)
    return () => document.removeEventListener("keydown", onKey)
  }, [])

  return (
    <div className="min-h-screen flex bg-background text-foreground">
      <Sidebar />
      <div className="flex-1 flex flex-col min-w-0">
        <TopBar onOpenPalette={() => setPaletteOpen(true)} />
        <main className="flex-1 p-8 overflow-y-auto">
          <Routes>
            <Route path="/" element={<Navigate to="/overview" replace />} />
            <Route path="/overview" element={<Overview />} />
            <Route path="/topology" element={<Topology />} />
            <Route path="/audit" element={<Audit />} />
            <Route path="/terminal" element={<Terminal />} />
            <Route path="/forecast" element={<Forecast />} />
            <Route path="/agent" element={<Agent />} />
            <Route path="/verify" element={<Verify />} />
            <Route path="/plan" element={<Plan />} />
            <Route path="/causes" element={<Causes />} />
            <Route path="/fleet" element={<Fleet />} />
            <Route path="/economics" element={<Economics />} />
            <Route path="/certificates" element={<Certificates />} />
          </Routes>
        </main>
      </div>
      <CommandPalette open={paletteOpen} setOpen={setPaletteOpen} />
    </div>
  )
}
