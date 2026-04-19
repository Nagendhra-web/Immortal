import { useState } from "react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Badge } from "@/components/ui/badge"
import { api } from "@/lib/api"

export default function Agent() {
  const [severity, setSeverity] = useState("warning")
  const [source, setSource] = useState("")
  const [message, setMessage] = useState("")
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<any>(null)
  const [error, setError] = useState<string | null>(null)

  async function run() {
    setLoading(true); setError(null); setResult(null)
    try {
      const res = await api.agenticRun({ severity, source, message })
      setResult(res)
    } catch (e: any) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  const steps: any[] = result?.steps ?? result?.trace ?? []

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-semibold">Agent Runner</h1>
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader><CardTitle>Run Agentic Event</CardTitle></CardHeader>
          <CardContent className="space-y-4">
            <div>
              <label className="text-sm font-medium">Severity</label>
              <Select value={severity} onValueChange={setSeverity}>
                <SelectTrigger className="mt-1"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="info">Info</SelectItem>
                  <SelectItem value="warning">Warning</SelectItem>
                  <SelectItem value="error">Error</SelectItem>
                  <SelectItem value="critical">Critical</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div>
              <label className="text-sm font-medium">Source</label>
              <Input className="mt-1" placeholder="service-name" value={source} onChange={e => setSource(e.target.value)} />
            </div>
            <div>
              <label className="text-sm font-medium">Message</label>
              <Textarea className="mt-1" placeholder="Describe the event…" value={message} onChange={e => setMessage(e.target.value)} rows={4} />
            </div>
            <Button onClick={run} disabled={loading} className="w-full">
              {loading ? "Running…" : "Run Agent"}
            </Button>
            {error && <p className="text-sm text-destructive">{error}</p>}
          </CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle>Trace</CardTitle></CardHeader>
          <CardContent>
            {!result && <p className="text-sm text-muted-foreground">Run an event to see trace steps.</p>}
            {steps.length > 0 && (
              <div className="space-y-3">
                {steps.map((s: any, i: number) => (
                  <div key={i} className="flex gap-3">
                    <div className="flex flex-col items-center">
                      <div className="h-6 w-6 rounded-full bg-primary text-primary-foreground flex items-center justify-center text-xs font-bold">{i + 1}</div>
                      {i < steps.length - 1 && <div className="w-0.5 flex-1 bg-border mt-1" />}
                    </div>
                    <div className="pb-3">
                      <p className="text-sm font-medium">{s.action ?? s.name ?? s.step}</p>
                      <p className="text-xs text-muted-foreground">{s.result ?? s.output ?? JSON.stringify(s)}</p>
                      {s.status && <Badge variant="outline" className="mt-1 text-xs">{s.status}</Badge>}
                    </div>
                  </div>
                ))}
              </div>
            )}
            {result && !steps.length && (
              <pre className="text-xs font-mono bg-muted rounded p-3 overflow-auto">{JSON.stringify(result, null, 2)}</pre>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
