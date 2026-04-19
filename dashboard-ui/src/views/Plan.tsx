import { useState } from "react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"
import { Badge } from "@/components/ui/badge"
import { api } from "@/lib/api"

export default function Plan() {
  const [input, setInput] = useState("")
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<any>(null)
  const [error, setError] = useState<string | null>(null)

  async function compile() {
    setLoading(true); setError(null); setResult(null)
    try {
      const res = await api.nlplan({ nl: input })
      setResult(res)
    } catch (e: any) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  const steps: any[] = result?.steps ?? result?.plan ?? []
  const safe = result?.safe ?? result?.safety?.ok ?? null

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-semibold">NL Plan Compiler</h1>
      <Card>
        <CardHeader><CardTitle>Natural Language Plan</CardTitle></CardHeader>
        <CardContent className="space-y-4">
          <Textarea
            rows={5}
            placeholder="Describe the remediation plan in plain language…"
            value={input}
            onChange={e => setInput(e.target.value)}
          />
          <Button onClick={compile} disabled={loading}>{loading ? "Compiling…" : "Compile"}</Button>
          {error && <p className="text-sm text-destructive">{error}</p>}
        </CardContent>
      </Card>
      {result && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-3">
              Compiled Plan
              {safe != null && <Badge variant={safe ? "default" : "destructive"}>{safe ? "SAFE" : "UNSAFE"}</Badge>}
            </CardTitle>
          </CardHeader>
          <CardContent>
            {steps.length > 0 ? (
              <ol className="space-y-2">
                {steps.map((s: any, i: number) => (
                  <li key={i} className="flex gap-3 text-sm">
                    <span className="font-bold text-primary shrink-0">{i + 1}.</span>
                    <span>{s.description ?? s.action ?? JSON.stringify(s)}</span>
                  </li>
                ))}
              </ol>
            ) : (
              <pre className="text-xs font-mono bg-muted rounded p-3 overflow-auto">{JSON.stringify(result, null, 2)}</pre>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  )
}
