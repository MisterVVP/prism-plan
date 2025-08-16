import { describe, it, expect } from "vitest";
import { tasksReducer, initialState } from "./tasksReducer";

describe("tasksReducer", () => {
  it("increments order per category", () => {
    const s1 = tasksReducer(initialState, {
      type: "add-task",
      taskId: "t1",
      commandId: "c1",
      partial: { title: "a", notes: "", category: "normal" },
    });
    const s2 = tasksReducer(s1, {
      type: "add-task",
      taskId: "t2",
      commandId: "c2",
      partial: { title: "b", notes: "", category: "normal" },
    });
    const s3 = tasksReducer(s2, {
      type: "add-task",
      taskId: "t3",
      commandId: "c3",
      partial: { title: "c", notes: "", category: "critical" },
    });
    const s4 = tasksReducer(s3, {
      type: "add-task",
      taskId: "t4",
      commandId: "c4",
      partial: { title: "d", notes: "", category: "critical" },
    });
    expect(s2.tasks[0].order).toBe(0);
    expect(s2.tasks[1].order).toBe(1);
    expect(s4.tasks[2].order).toBe(0);
    expect(s4.tasks[3].order).toBe(1);
  });

  it("queues matching commands", () => {
    const s1 = tasksReducer(initialState, {
      type: "add-task",
      taskId: "t1",
      commandId: "c1",
      partial: { title: "x", notes: "", category: "fun" },
    });
    expect(s1.commands).toHaveLength(1);
    expect((s1.commands[0].data as any).order).toBe(0);
  });
});
