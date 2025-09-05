import { describe, it, expect } from "vitest";
import { settingsReducer, settingsInitialState } from ".";

describe("settingsReducer", () => {
  it("maps returned idempotency keys only to commands missing them", () => {
    const s1 = settingsReducer(settingsInitialState, {
      type: "update-settings",
      userId: "u1",
      settings: { tasksPerCategory: 5 },
    });
    const s2 = settingsReducer(s1, {
      type: "update-settings",
      userId: "u1",
      settings: { showDoneTasks: true },
    });
    const s3 = {
      ...s2,
      commands: [{ ...s2.commands[0], idempotencyKey: "k1" }, s2.commands[1]],
    };
    const s4 = settingsReducer(s3, {
      type: "set-idempotency-keys",
      keys: ["k2"],
    });
    expect(s4.commands[0].idempotencyKey).toBe("k1");
    expect(s4.commands[1].idempotencyKey).toBe("k2");
  });
});
