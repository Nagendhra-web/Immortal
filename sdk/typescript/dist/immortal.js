"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.Immortal = void 0;
class Immortal {
    constructor(config = {}) {
        this.rules = [];
        this.running = false;
        this.events = [];
        this.config = {
            name: config.name ?? "immortal-app",
            mode: config.mode ?? "reactive",
            endpoint: config.endpoint ?? "http://localhost:7777",
            ghostMode: config.ghostMode ?? false,
        };
    }
    addRule(rule) {
        this.rules.push(rule);
    }
    heal(rule) {
        this.addRule({
            name: rule.name,
            match: rule.when,
            action: rule.do,
        });
    }
    async handleEvent(event) {
        this.events.push(event);
        const matched = [];
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
    start() {
        this.running = true;
    }
    stop() {
        this.running = false;
    }
    isRunning() {
        return this.running;
    }
    getConfig() {
        return { ...this.config };
    }
    getEvents() {
        return [...this.events];
    }
}
exports.Immortal = Immortal;
