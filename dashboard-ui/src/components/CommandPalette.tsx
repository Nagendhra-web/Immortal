import { useEffect } from "react"
import { useNavigate } from "react-router-dom"
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from "@/components/ui/command"
import { useTheme } from "@/components/ThemeProvider"

interface Props {
  open: boolean
  setOpen: (v: boolean) => void
}

const NAV_ITEMS = [
  { label: "Overview", path: "/overview" },
  { label: "Topology", path: "/topology" },
  { label: "Audit", path: "/audit" },
  { label: "Terminal", path: "/terminal" },
  { label: "Forecast", path: "/forecast" },
  { label: "Agent Runner", path: "/agent" },
  { label: "Formal Verify", path: "/verify" },
  { label: "NL Plan", path: "/plan" },
  { label: "Root Causes", path: "/causes" },
  { label: "Fleet", path: "/fleet" },
  { label: "Economics", path: "/economics" },
  { label: "Certificates", path: "/certificates" },
]

export default function CommandPalette({ open, setOpen }: Props) {
  const navigate = useNavigate()
  const { theme, setTheme } = useTheme()

  useEffect(() => {
    function handler(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault()
        setOpen(true)
      }
      if (e.key === "/" && document.activeElement?.tagName === "BODY") {
        e.preventDefault()
        setOpen(true)
      }
    }
    document.addEventListener("keydown", handler)
    return () => document.removeEventListener("keydown", handler)
  }, [setOpen])

  function go(path: string) {
    navigate(path)
    setOpen(false)
  }

  function copyNodeId() {
    navigator.clipboard.writeText(window.location.hostname)
    setOpen(false)
  }

  function copyUrl() {
    navigator.clipboard.writeText(window.location.href)
    setOpen(false)
  }

  return (
    <CommandDialog open={open} onOpenChange={setOpen}>
      <CommandInput placeholder="Search commands…" />
      <CommandList>
        <CommandEmpty>No results found.</CommandEmpty>
        <CommandGroup heading="Navigate">
          {NAV_ITEMS.map(item => (
            <CommandItem key={item.path} onSelect={() => go(item.path)}>
              {item.label}
            </CommandItem>
          ))}
        </CommandGroup>
        <CommandSeparator />
        <CommandGroup heading="Actions">
          <CommandItem onSelect={() => { setTheme(theme === "dark" ? "light" : "dark"); setOpen(false) }}>
            Toggle Theme ({theme === "dark" ? "→ Light" : "→ Dark"})
          </CommandItem>
          <CommandItem onSelect={() => { window.location.reload(); setOpen(false) }}>
            Reset / Reload
          </CommandItem>
        </CommandGroup>
        <CommandSeparator />
        <CommandGroup heading="Data">
          <CommandItem onSelect={copyNodeId}>Copy Node ID</CommandItem>
          <CommandItem onSelect={copyUrl}>Copy Current URL</CommandItem>
        </CommandGroup>
      </CommandList>
    </CommandDialog>
  )
}
