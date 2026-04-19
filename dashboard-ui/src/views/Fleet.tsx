import { useState } from "react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"
import { Badge } from "@/components/ui/badge"
import { api } from "@/lib/api"

export default function Fleet() {
  const [incident, setIncident] = useState("")
  const [loading, setLoading] = useState(false)
  const [recs, setRecs] = useState<any[]>([])
  const [error, setError] = useState<string | null>(null)

  async function analyze() {
    setLoading(true); setError(null)
    try {
      const res = await api.recommendations()
      setRecs(res?.recommendations ?? res ?? [])
    } catch (e: any) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  const priorityColor: Record<string, any> = {
    high: "destructive",
    medium: "default",
    low: "outline",
  }

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-semibold">Fleet</h1>
      <Card>
        <CardHeader><CardTitle>Describe Incident</CardTitle></CardHeader>
        <CardContent className="space-y-4">
          <Textarea
            rows={4}
            placeholder="Describe the incident or anomaly to get fleet-wide recommendations…"
            value={incident}
            onChange={e => setIncident(e.target.value)}
          />
          <Button onClick={analyze} disabled={loading}>{loading ? "Analyzing…" : "Get Recommendations"}</Button>
          {error && <p className="text-sm text-destructive">{error}</p>}
        </CardContent>
      </Card>
      {recs.length > 0 && (
        <div className="space-y-3">
          {recs.map((r: any, i: number) => (
            <Card key={i}>
              <CardContent className="pt-4">
                <div className="flex items-start justify-between gap-4">
                  <div>
                    <p className="font-medium">{r.title ?? r.action ?? r.recommendation}</p>
                    <p className="text-sm text-muted-foreground mt-1">{r.description ?? r.reason ?? ""}</p>
                  </div>
                  <Badge variant={priorityColor[r.priority] ?? "outline"}>{r.priority ?? "info"}</Badge>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
      {!recs.length && !loading && (
        <p className="text-muted-foreground text-sm">Submit an incident description to see recommendations.</p>
      )}
    </div>
  )
}
