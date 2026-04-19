import { useState } from "react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"

const DEFAULT_SERVICES = [
  { name: "auth-service", costPerHour: "12.50", revenuePerHour: "380.00" },
  { name: "payment-service", costPerHour: "24.00", revenuePerHour: "1200.00" },
  { name: "api-gateway", costPerHour: "8.00", revenuePerHour: "620.00" },
  { name: "ml-inference", costPerHour: "45.00", revenuePerHour: "290.00" },
]

export default function Economics() {
  const [services, setServices] = useState(DEFAULT_SERVICES)
  const [target, setTarget] = useState("auth-service")
  const [loading, setLoading] = useState(false)
  const [netValue, setNetValue] = useState<number | null>(null)

  function updateService(i: number, field: string, value: string) {
    setServices(prev => prev.map((s, idx) => idx === i ? { ...s, [field]: value } : s))
  }

  async function simulate() {
    setLoading(true)
    await new Promise(r => setTimeout(r, 600))
    const svc = services.find(s => s.name === target)
    if (!svc) { setLoading(false); return }
    const cost = parseFloat(svc.costPerHour) * 24
    const revenue = parseFloat(svc.revenuePerHour) * 24
    setNetValue(revenue - cost)
    setLoading(false)
  }

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-semibold">Economics</h1>
      <Card>
        <CardHeader><CardTitle>Service Cost Model</CardTitle></CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Service</TableHead>
                <TableHead>Cost/hr ($)</TableHead>
                <TableHead>Revenue/hr ($)</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {services.map((s, i) => (
                <TableRow key={i}>
                  <TableCell className="font-mono text-sm">{s.name}</TableCell>
                  <TableCell>
                    <Input
                      className="w-24 h-7 text-sm"
                      value={s.costPerHour}
                      onChange={e => updateService(i, "costPerHour", e.target.value)}
                    />
                  </TableCell>
                  <TableCell>
                    <Input
                      className="w-24 h-7 text-sm"
                      value={s.revenuePerHour}
                      onChange={e => updateService(i, "revenuePerHour", e.target.value)}
                    />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
      <Card>
        <CardHeader><CardTitle>Simulate Incident</CardTitle></CardHeader>
        <CardContent className="space-y-4">
          <div className="flex gap-3 items-end">
            <div className="flex-1">
              <label className="text-sm font-medium">Incident Target</label>
              <Select value={target} onValueChange={setTarget}>
                <SelectTrigger className="mt-1"><SelectValue /></SelectTrigger>
                <SelectContent>
                  {services.map(s => <SelectItem key={s.name} value={s.name}>{s.name}</SelectItem>)}
                </SelectContent>
              </Select>
            </div>
            <Button onClick={simulate} disabled={loading}>{loading ? "Simulating…" : "Simulate"}</Button>
          </div>
          {netValue != null && (
            <div className="rounded-lg border p-4">
              <p className="text-sm text-muted-foreground">24h Net Value (revenue − cost)</p>
              <p className={`text-4xl font-bold font-mono mt-1 ${netValue >= 0 ? "text-green-600" : "text-red-600"}`}>
                ${netValue.toLocaleString("en-US", { minimumFractionDigits: 2 })}
              </p>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
