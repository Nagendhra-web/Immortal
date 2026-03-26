import { ImmortalEvent, Severity } from "./event";
import * as crypto from "crypto";

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

export class Immortal {
  private config: Required<ImmortalConfig>;
  private rules: HealingRule[] = [];
  private running = false;
  private events: ImmortalEvent[] = [];

  constructor(config: ImmortalConfig = {}) {
    this.config = {
      name: config.name ?? "immortal-app",
      mode: config.mode ?? "reactive",
      endpoint: config.endpoint ?? "http://localhost:7777",
      ghostMode: config.ghostMode ?? false,
    };
  }

  addRule(rule: HealingRule): void {
    this.rules.push(rule);
  }

  heal(rule: {
    name: string;
    when: (event: ImmortalEvent) => boolean;
    do: (event: ImmortalEvent) => Promise<void>;
  }): void {
    this.addRule({
      name: rule.name,
      match: rule.when,
      action: rule.do,
    });
  }

  async handleEvent(event: ImmortalEvent): Promise<string[]> {
    this.events.push(event);
    const matched: string[] = [];

    for (const rule of this.rules) {
      if (rule.match(event)) {
        matched.push(rule.name);
        if (!this.config.ghostMode) {
          await rule.action(event);
        }
      }
    }

    return matched;
  }

  start(): void {
    this.running = true;
  }

  stop(): void {
    this.running = false;
  }

  isRunning(): boolean {
    return this.running;
  }

  getConfig(): Required<ImmortalConfig> {
    return { ...this.config };
  }

  getEvents(): ImmortalEvent[] {
    return [...this.events];
  }
}
