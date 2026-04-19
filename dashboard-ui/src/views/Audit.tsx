import { useQuery } from "@tanstack/react-query"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { api } from "@/lib/api"

export default function Audit() {
  const { data: verify } = useQuery({ queryKey: ["auditVerify"], queryFn: api.auditVerify, retry: false })
  const { data: entries } = useQuery({ queryKey: ["auditEntries"], queryFn: () => api.auditEntries(20), retry: false })

  const verified = verify?.verified ?? verify?.ok ?? false
  const list: any[] = entries?.entries ?? entries ?? []

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Audit</h1>
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-3">
            Chain Integrity
            <Badge variant={verified ? "default" : "destructive"} className="text-base px-3 py-1">
              {verified ? "VERIFIED" : "UNVERIFIED"}
            </Badge>
          </CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">
            {verify?.message ?? (verified ? "All audit chain hashes valid." : "Chain verification status unavailable.")}
          </p>
        </CardContent>
      </Card>
      <Card>
        <CardHeader><CardTitle>Audit Entries</CardTitle></CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>ID</TableHead>
                <TableHead>Timestamp</TableHead>
                <TableHead>Action</TableHead>
                <TableHead>Actor</TableHead>
                <TableHead>Hash</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {list.map((e: any, i: number) => (
                <TableRow key={i}>
                  <TableCell className="font-mono text-xs">{e.id ?? i}</TableCell>
                  <TableCell className="font-mono text-xs">{e.timestamp ?? e.time ?? "—"}</TableCell>
                  <TableCell>{e.action ?? e.event ?? "—"}</TableCell>
                  <TableCell>{e.actor ?? e.source ?? "—"}</TableCell>
                  <TableCell className="font-mono text-xs truncate max-w-[120px]">{e.hash ?? "—"}</TableCell>
                </TableRow>
              ))}
              {!list.length && (
                <TableRow><TableCell colSpan={5} className="text-center text-muted-foreground">No entries</TableCell></TableRow>
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  )
}
