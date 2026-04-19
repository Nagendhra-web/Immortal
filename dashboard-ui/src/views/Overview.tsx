import { useQuery } from "@tanstack/react-query"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { api } from "@/lib/api"

export default function Overview() {
  const { data: status } = useQuery({ queryKey: ["status"], queryFn: api.status })
  const { data: events } = useQuery({ queryKey: ["events"], queryFn: () => api.events(20) })
  const { data: deps } = useQuery({ queryKey: ["dependencies"], queryFn: api.dependencies })

  const uptime = status?.uptime ?? "—"
  const eventCount = status?.events_processed ?? events?.length ?? "—"
  const healingActions = status?.healing_actions ?? "—"
  const servicesHealthy = deps?.services?.filter((s: any) => s.status === "healthy").length ?? "—"
  const totalServices = deps?.services?.length ?? "—"

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Overview</h1>
      <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <StatCard title="Uptime" value={uptime} />
        <StatCard title="Events Processed" value={String(eventCount)} />
        <StatCard title="Healing Actions" value={String(healingActions)} />
        <StatCard title="Services Healthy" value={`${servicesHealthy}/${totalServices}`} />
      </div>
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader><CardTitle>Live Feed</CardTitle></CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Time</TableHead>
                  <TableHead>Source</TableHead>
                  <TableHead>Message</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(events?.events ?? events ?? []).slice(0, 10).map((e: any, i: number) => (
                  <TableRow key={i}>
                    <TableCell className="font-mono text-xs">{e.timestamp ?? e.time ?? "—"}</TableCell>
                    <TableCell><Badge variant="outline">{e.source ?? e.service ?? "system"}</Badge></TableCell>
                    <TableCell className="max-w-xs truncate">{e.message ?? e.msg ?? JSON.stringify(e)}</TableCell>
                  </TableRow>
                ))}
                {!(events?.events ?? events)?.length && (
                  <TableRow><TableCell colSpan={3} className="text-center text-muted-foreground">No events</TableCell></TableRow>
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle>Services</CardTitle></CardHeader>
          <CardContent>
            <div className="grid grid-cols-2 gap-2">
              {(deps?.services ?? []).map((s: any, i: number) => (
                <div key={i} className="flex items-center justify-between rounded border p-2 text-sm">
                  <span>{s.name}</span>
                  <Badge variant={s.status === "healthy" ? "default" : "destructive"}>{s.status}</Badge>
                </div>
              ))}
              {!deps?.services?.length && <p className="text-muted-foreground text-sm">No service data</p>}
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

function StatCard({ title, value }: { title: string; value: string }) {
  return (
    <Card>
      <CardHeader className="pb-2"><CardTitle className="text-sm font-medium text-muted-foreground">{title}</CardTitle></CardHeader>
      <CardContent><p className="text-2xl font-bold font-mono">{value}</p></CardContent>
    </Card>
  )
}
