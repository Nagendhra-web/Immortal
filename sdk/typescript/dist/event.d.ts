export type EventType = "error" | "metric" | "log" | "trace" | "health";
export type Severity = "debug" | "info" | "warning" | "error" | "critical" | "fatal";
export interface ImmortalEvent {
    id: string;
    type: EventType;
    severity: Severity;
    message: string;
    source: string;
    timestamp: Date;
    meta: Record<string, unknown>;
}
