import { useState } from "react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"
import { Badge } from "@/components/ui/badge"
import { api } from "@/lib/api"

export default function Verify() {
  const [world, setWorld] = useState("")
  const [plan, setPlan] = useState("")
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<any>(null)
  const [error, setError] = useState<string | null>(null)

  async function verify() {
    setLoading(true); setError(null); setResult(null)
    try {
      const res = await api.formalCheck({ world, plan })
      setResult(res)
    } catch (e: any) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  const safe = result?.safe ?? result?.ok ?? result?.verified
  const invariants: any[] = result?.invariants ?? []

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-semibold">Formal Verify</h1>
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader><CardTitle>Input</CardTitle></CardHeader>
          <CardContent className="space-y-4">
            <div>
              <label className="text-sm font-medium">World State (JSON)</label>
              <Textarea className="mt-1 font-mono text-xs" rows={6} placeholder='{"services": [...]}' value={world} onChange={e => setWorld(e.target.value)} />
            </div>
            <div>
              <label className="text-sm font-medium">Plan (JSON or text)</label>
              <Textarea className="mt-1 font-mono text-xs" rows={6} placeholder='{"steps": [...]}' value={plan} onChange={e => setPlan(e.target.value)} />
            </div>
            <Button onClick={verify} disabled={loading} className="w-full">
              {loading ? "Verifying…" : "Verify"}
            </Button>
            {error && <p className="text-sm text-destructive">{error}</p>}
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-3">
              Result
              {result != null && (
                <Badge variant={safe ? "default" : "destructive"}>
                  {safe ? "SAFE" : "UNSAFE"}
                </Badge>
              )}
            </CardTitle>
          </CardHeader>
          <CardContent>
            {!result && <p className="text-sm text-muted-foreground">Run verification to see results.</p>}
            {result && (
              <div className="space-y-3">
                {result.message && <p className="text-sm">{result.message}</p>}
                {invariants.length > 0 && (
                  <div>
                    <p className="text-sm font-medium mb-2">Invariants</p>
                    <div className="space-y-1">
                      {invariants.map((inv: any, i: number) => (
                        <div key={i} className="flex items-center gap-2 text-sm">
                          <Badge variant={inv.holds ? "default" : "destructive"} className="text-xs">{inv.holds ? "holds" : "violated"}</Badge>
                          <span>{inv.name ?? inv.description ?? JSON.stringify(inv)}</span>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
                {!invariants.length && (
                  <pre className="text-xs font-mono bg-muted rounded p-3 overflow-auto">{JSON.stringify(result, null, 2)}</pre>
                )}
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
