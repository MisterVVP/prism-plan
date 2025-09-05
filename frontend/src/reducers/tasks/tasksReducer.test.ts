import { describe, it, expect } from "vitest";
import { tasksReducer, initialState } from ".";

describe("tasksReducer", () => {
  it("increments order per category", () => {
    const s1 = tasksReducer(initialState, {
      type: "add-task",
      partial: { title: "a", notes: "", category: "normal" },
    });
    const s2 = tasksReducer(s1, {
      type: "add-task",
      partial: { title: "b", notes: "", category: "normal" },
    });
    const s3 = tasksReducer(s2, {
      type: "add-task",
      partial: { title: "c", notes: "", category: "critical" },
    });
    const s4 = tasksReducer(s3, {
      type: "add-task",
      partial: { title: "d", notes: "", category: "critical" },
    });
    expect((s4.commands[0].data as any).order).toBe(0);
    expect((s4.commands[1].data as any).order).toBe(1);
    expect((s4.commands[2].data as any).order).toBe(0);
    expect((s4.commands[3].data as any).order).toBe(1);
  });

  it("queues matching commands", () => {
    const s1 = tasksReducer(initialState, {
      type: "add-task",
      partial: { title: "x", notes: "", category: "fun" },
    });
    expect(s1.commands).toHaveLength(1);
    expect((s1.commands[0].data as any).order).toBe(0);
  });

  it("keeps order after remote reset", () => {
    const s1 = tasksReducer(initialState, {
      type: "add-task",
      partial: { title: "a", notes: "", category: "normal" },
    });
    const s2 = tasksReducer(s1, { type: "clear-commands" });
    const s3 = tasksReducer(s2, { type: "set-tasks", tasks: [] });
    const s4 = tasksReducer(s3, {
      type: "add-task",
      partial: { title: "b", notes: "", category: "normal" },
    });
    expect((s4.commands[0].data as any).order).toBe(1);
  });

  it("merges streamed tasks", () => {
    const s1 = tasksReducer(initialState, {
      type: "set-tasks",
      tasks: [
        { id: "t1", title: "a", notes: "", category: "normal", order: 0 },
      ],
    });
    const s2 = tasksReducer(s1, {
      type: "merge-tasks",
      tasks: [
        { id: "t2", title: "b", notes: "", category: "normal", order: 1 },
      ],
    });
    expect(s2.tasks).toHaveLength(2);
    const s3 = tasksReducer(s2, {
      type: "merge-tasks",
      tasks: [
        {
          id: "t1",
          title: "a2",
          notes: "",
          category: "normal",
          order: 0,
          done: true,
        },
      ],
    });
    const t1 = s3.tasks.find((t) => t.id === "t1");
    expect(t1?.title).toBe("a2");
    expect(t1?.done).toBe(true);
  });

  it("updates task fields", () => {
    const s1 = tasksReducer(initialState, {
      type: "set-tasks",
      tasks: [{ id: "t1", title: "a", notes: "", category: "normal", order: 0 }],
    });
    const s2 = tasksReducer(s1, {
      type: "update-task",
      id: "t1",
      changes: { title: "b" },
    });
    expect(s2.tasks[0].title).toBe("b");
    expect(s2.commands[0]).toMatchObject({
      type: "update-task",
      data: { id: "t1", title: "b" },
    });
  });

  it("completes task and queues command", () => {
    const s1 = tasksReducer(initialState, {
      type: "set-tasks",
      tasks: [{ id: "t1", title: "a", notes: "", category: "normal", order: 0 }],
    });
    const s2 = tasksReducer(s1, {
      type: "complete-task",
      id: "t1",
    });
    expect(s2.tasks[0].done).toBe(true);
    expect(s2.commands[0]).toMatchObject({
      type: "complete-task",
      data: { id: "t1" },
    });
  });

  it("maps returned idempotency keys only to commands missing them", () => {
    const s1 = tasksReducer(initialState, {
      type: "add-task",
      partial: { title: "a", notes: "", category: "normal" },
    });
    const s2 = tasksReducer(s1, {
      type: "add-task",
      partial: { title: "b", notes: "", category: "normal" },
    });
    const s3 = {
      ...s2,
      commands: [{ ...s2.commands[0], idempotencyKey: "k1" }, s2.commands[1]],
    };
    const s4 = tasksReducer(s3, {
      type: "set-idempotency-keys",
      keys: ["k2"],
    });
    expect(s4.commands[0].idempotencyKey).toBe("k1");
    expect(s4.commands[1].idempotencyKey).toBe("k2");
  });
});
