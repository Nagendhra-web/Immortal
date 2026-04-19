import { useQuery } from "@tanstack/react-query"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"

async function fetchCerts() {
  const r = await fetch("/api/v5/certificates")
  if (!r.ok) return null
  return r.json()
}

const MOCK_CERTS = [
  { id: "cert-001", name: "api-gateway-tls", expires: "2026-01-15", status: "valid", issuer: "Let's Encrypt" },
  { id: "cert-002", name: "internal-ca", expires: "2027-06-30", status: "valid", issuer: "Internal CA" },
  { id: "cert-003", name: "payment-mtls", expires: "2025-11-01", status: "expiring", issuer: "DigiCert" },
]

export default function Certificates() {
  const { data } = useQuery({ queryKey: ["certs"], queryFn: fetchCerts, retry: false })
  const certs: any[] = data?.certificates ?? data ?? MOCK_CERTS

  function download(cert: any) {
    const blob = new Blob([JSON.stringify(cert, null, 2)], { type: "application/json" })
    const url = URL.createObjectURL(blob)
    const a = document.createElement("a")
    a.href = url; a.download = `${cert.id ?? cert.name}.json`
    a.click(); URL.revokeObjectURL(url)
  }

  const statusVariant: Record<string, any> = {
    valid: "default",
    expiring: "destructive",
    expired: "destructive",
    revoked: "destructive",
  }

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-semibold">Certificates</h1>
      <Card>
        <CardHeader><CardTitle>TLS / mTLS Certificates</CardTitle></CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Issuer</TableHead>
                <TableHead>Expires</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Download</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {certs.map((c: any, i: number) => (
                <TableRow key={i}>
                  <TableCell className="font-mono text-sm">{c.name ?? c.id}</TableCell>
                  <TableCell>{c.issuer ?? "—"}</TableCell>
                  <TableCell className="font-mono text-sm">{c.expires ?? c.expiry ?? "—"}</TableCell>
                  <TableCell>
                    <Badge variant={statusVariant[c.status] ?? "outline"}>{c.status ?? "unknown"}</Badge>
                  </TableCell>
                  <TableCell>
                    <Button variant="outline" size="sm" onClick={() => download(c)}>JSON</Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  )
}
