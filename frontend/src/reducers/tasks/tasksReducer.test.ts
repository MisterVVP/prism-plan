import { describe, it, expect } from "vitest";
import { tasksReducer, initialState } from ".";

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

  it("keeps order after remote reset", () => {
    const s1 = tasksReducer(initialState, {
      type: "add-task",
      taskId: "t1",
      commandId: "c1",
      partial: { title: "a", notes: "", category: "normal" },
    });
    const s2 = tasksReducer(s1, { type: "clear-commands" });
    const s3 = tasksReducer(s2, { type: "set-tasks", tasks: [] });
    const s4 = tasksReducer(s3, {
      type: "add-task",
      taskId: "t2",
      commandId: "c2",
      partial: { title: "b", notes: "", category: "normal" },
    });
    expect(s4.tasks[0].order).toBe(1);
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
      type: "add-task",
      taskId: "t1",
      commandId: "c1",
      partial: { title: "a", notes: "", category: "normal" },
    });
    const s2 = tasksReducer(s1, {
      type: "update-task",
      id: "t1",
      commandId: "c2",
      changes: { title: "b" },
    });
    expect(s2.tasks[0].title).toBe("b");
    expect(s2.commands[1]).toMatchObject({ type: "update-task", entityId: "t1" });
  });

  it("completes task and queues command", () => {
    const s1 = tasksReducer(initialState, {
      type: "add-task",
      taskId: "t1",
      commandId: "c1",
      partial: { title: "a", notes: "", category: "normal" },
    });
    const s2 = tasksReducer(s1, {
      type: "complete-task",
      id: "t1",
      commandId: "c2",
    });
    expect(s2.tasks[0].done).toBe(true);
    expect(s2.commands[1]).toMatchObject({ type: "complete-task", entityId: "t1" });
  });
});
