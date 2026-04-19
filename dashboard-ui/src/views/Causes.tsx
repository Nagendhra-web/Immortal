import { useState } from "react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from "recharts"
import { api } from "@/lib/api"

export default function Causes() {
  const [outcome, setOutcome] = useState("")
  const [variables, setVariables] = useState("")
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<any>(null)
  const [error, setError] = useState<string | null>(null)
  const [activeTab, setActiveTab] = useState("pc")

  async function run() {
    setLoading(true); setError(null); setResult(null)
    try {
      const vars = variables.split(",").map(v => v.trim()).filter(Boolean)
      const body = { outcome, variables: vars }
      const res = activeTab === "pc"
        ? await api.causalRootCause(body)
        : await api.pcmci(body)
      setResult(res)
    } catch (e: any) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  const chartData: any[] = (result?.causes ?? result?.effects ?? []).map((c: any) => ({
    name: c.variable ?? c.name,
    ace: Math.abs(c.ace ?? c.effect ?? c.score ?? 0),
  }))

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-semibold">Causal Root Cause</h1>
      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <TabsList>
          <TabsTrigger value="pc">PC Algorithm</TabsTrigger>
          <TabsTrigger value="pcmci">PCMCI</TabsTrigger>
        </TabsList>
        <TabsContent value="pc">
          <Card>
            <CardHeader><CardTitle>PC Causal Discovery</CardTitle></CardHeader>
            <CardContent className="space-y-4">
              <InputForm outcome={outcome} setOutcome={setOutcome} variables={variables} setVariables={setVariables} loading={loading} onRun={run} error={error} />
            </CardContent>
          </Card>
        </TabsContent>
        <TabsContent value="pcmci">
          <Card>
            <CardHeader><CardTitle>PCMCI Time-Series Causality</CardTitle></CardHeader>
            <CardContent className="space-y-4">
              <InputForm outcome={outcome} setOutcome={setOutcome} variables={variables} setVariables={setVariables} loading={loading} onRun={run} error={error} />
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
      {chartData.length > 0 && (
        <Card>
          <CardHeader><CardTitle>Average Causal Effect (ACE)</CardTitle></CardHeader>
          <CardContent>
            <ResponsiveContainer width="100%" height={280}>
              <BarChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="name" tick={{ fontSize: 11 }} />
                <YAxis tick={{ fontSize: 11 }} />
                <Tooltip />
                <Bar dataKey="ace" fill="#6366f1" />
              </BarChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>
      )}
      {result && !chartData.length && (
        <Card>
          <CardHeader><CardTitle>Result</CardTitle></CardHeader>
          <CardContent>
            <pre className="text-xs font-mono bg-muted rounded p-3 overflow-auto">{JSON.stringify(result, null, 2)}</pre>
          </CardContent>
        </Card>
      )}
    </div>
  )
}

function InputForm({ outcome, setOutcome, variables, setVariables, loading, onRun, error }: any) {
  return (
    <>
      <div>
        <label className="text-sm font-medium">Outcome variable</label>
        <Input className="mt-1" placeholder="e.g. latency_p99" value={outcome} onChange={e => setOutcome(e.target.value)} />
      </div>
      <div>
        <label className="text-sm font-medium">Variables (comma-separated)</label>
        <Input className="mt-1" placeholder="cpu_usage, mem_rss, error_rate" value={variables} onChange={e => setVariables(e.target.value)} />
      </div>
      <Button onClick={onRun} disabled={loading}>{loading ? "Running…" : "Analyze"}</Button>
      {error && <p className="text-sm text-destructive">{error}</p>}
    </>
  )
}
