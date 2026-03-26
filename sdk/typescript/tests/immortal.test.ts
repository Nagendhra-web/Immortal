import { Immortal, ImmortalEvent } from "../src";

describe("Immortal SDK", () => {
  it("creates with default config", () => {
    const app = new Immortal();
    const config = app.getConfig();
    expect(config.name).toBe("immortal-app");
    expect(config.mode).toBe("reactive");
    expect(config.ghostMode).toBe(false);
  });

  it("creates with custom config", () => {
    const app = new Immortal({ name: "my-api", mode: "autonomous" });
    expect(app.getConfig().name).toBe("my-api");
    expect(app.getConfig().mode).toBe("autonomous");
  });

  it("starts and stops", () => {
    const app = new Immortal();
    app.start();
    expect(app.isRunning()).toBe(true);
    app.stop();
    expect(app.isRunning()).toBe(false);
  });

  it("matches and executes healing rules", async () => {
    const app = new Immortal();
    let healed = false;

    app.heal({
      name: "restart-on-crash",
      when: (e) => e.severity === "critical",
      do: async () => {
        healed = true;
      },
    });

    const event: ImmortalEvent = {
      id: "test-1",
      type: "error",
      severity: "critical",
      message: "process crashed",
      source: "test",
      timestamp: new Date(),
      meta: {},
    };

    const matched = await app.handleEvent(event);
    expect(matched).toContain("restart-on-crash");
    expect(healed).toBe(true);
  });

  it("ghost mode does not execute actions", async () => {
    const app = new Immortal({ ghostMode: true });
    let executed = false;

    app.heal({
      name: "test-rule",
      when: (e) => e.severity === "critical",
      do: async () => {
        executed = true;
      },
    });

    const event: ImmortalEvent = {
      id: "test-2",
      type: "error",
      severity: "critical",
      message: "crash",
      source: "test",
      timestamp: new Date(),
      meta: {},
    };

    const matched = await app.handleEvent(event);
    expect(matched).toContain("test-rule");
    expect(executed).toBe(false);
  });

  it("stores events", async () => {
    const app = new Immortal();
    const event: ImmortalEvent = {
      id: "test-3",
      type: "error",
      severity: "error",
      message: "something broke",
      source: "test",
      timestamp: new Date(),
      meta: {},
    };

    await app.handleEvent(event);
    expect(app.getEvents()).toHaveLength(1);
    expect(app.getEvents()[0].message).toBe("something broke");
  });
});
