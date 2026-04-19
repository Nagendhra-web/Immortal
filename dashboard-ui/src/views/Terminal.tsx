import { useQuery } from "@tanstack/react-query"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { api } from "@/lib/api"

export default function Terminal() {
  const { data: events } = useQuery({ queryKey: ["events-term"], queryFn: () => api.events(50) })
  const list: any[] = events?.events ?? events ?? []

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-semibold">Terminal</h1>
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <span className="h-3 w-3 rounded-full bg-red-500 inline-block" />
            <span className="h-3 w-3 rounded-full bg-yellow-500 inline-block" />
            <span className="h-3 w-3 rounded-full bg-green-500 inline-block" />
            <span className="ml-2 text-sm font-mono text-muted-foreground">immortal — event log</span>
          </CardTitle>
        </CardHeader>
        <CardContent>
          <pre className="bg-zinc-950 text-green-400 rounded-lg p-4 font-mono text-xs overflow-auto max-h-[560px] whitespace-pre-wrap">
            {list.length
              ? list.map((e: any) =>
                  `[${e.timestamp ?? e.time ?? "—"}] [${e.source ?? e.service ?? "sys"}] ${e.message ?? e.msg ?? JSON.stringify(e)}\n`
                ).join("")
              : "Waiting for events…"}
          </pre>
        </CardContent>
      </Card>
    </div>
  )
}
