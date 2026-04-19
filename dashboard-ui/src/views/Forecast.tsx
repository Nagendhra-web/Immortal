import { useQuery } from "@tanstack/react-query"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Area, AreaChart } from "recharts"

async function fetchForecast() {
  const r = await fetch("/api/v5/forecast")
  if (!r.ok) return null
  return r.json()
}

export default function Forecast() {
  const { data } = useQuery({ queryKey: ["forecast"], queryFn: fetchForecast, retry: false })
  const points: any[] = data?.points ?? data?.forecast ?? []

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-semibold">Forecast</h1>
      <Card>
        <CardHeader><CardTitle>P95 Latency Forecast</CardTitle></CardHeader>
        <CardContent>
          {!points.length ? (
            <p className="text-muted-foreground text-sm py-8 text-center">
              No forecast data available — /api/v5/forecast endpoint not present.
            </p>
          ) : (
            <ResponsiveContainer width="100%" height={320}>
              <AreaChart data={points}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="t" tick={{ fontSize: 11 }} />
                <YAxis tick={{ fontSize: 11 }} />
                <Tooltip />
                <Area type="monotone" dataKey="p95" stroke="#6366f1" fill="#6366f133" strokeWidth={2} name="P95" />
                <Line type="monotone" dataKey="mean" stroke="#10b981" strokeWidth={2} dot={false} name="Mean" />
              </AreaChart>
            </ResponsiveContainer>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
