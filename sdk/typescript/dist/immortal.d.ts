import { ImmortalEvent } from "./event";
export type HealingMode = "ghost" | "reactive" | "predictive" | "autonomous";
export interface HealingRule {
    name: string;
    match: (event: ImmortalEvent) => boolean;
    action: (event: ImmortalEvent) => Promise<void>;
}
export interface ImmortalConfig {
    name?: string;
    mode?: HealingMode;
    endpoint?: string;
    ghostMode?: boolean;
}
export declare class Immortal {
    private config;
    private rules;
    private running;
    private events;
    constructor(config?: ImmortalConfig);
    addRule(rule: HealingRule): void;
    heal(rule: {
        name: string;
        when: (event: ImmortalEvent) => boolean;
        do: (event: ImmortalEvent) => Promise<void>;
    }): void;
    handleEvent(event: ImmortalEvent): Promise<string[]>;
    start(): void;
    stop(): void;
    isRunning(): boolean;
    getConfig(): Required<ImmortalConfig>;
    getEvents(): ImmortalEvent[];
}
