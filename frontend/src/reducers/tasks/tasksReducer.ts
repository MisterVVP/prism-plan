import type { Task, Command } from "../types";

type Counters = Record<Task["category"], number>;

type State = {
  tasks: Task[];
  commands: Command[];
  nextOrder: Counters;
};

const initialState: State = {
  tasks: [],
  commands: [],
  nextOrder: { critical: 0, fun: 0, important: 0, normal: 0 },
};

type AddTaskAction = {
  type: "add-task";
  taskId: string;
  commandId: string;
  partial: Omit<Task, "id" | "order" | "done">;
};

type UpdateTaskAction = {
  type: "update-task";
  id: string;
  commandId: string;
  changes: Partial<Task>;
};

type CompleteTaskAction = {
  type: "complete-task";
  id: string;
  commandId: string;
};

type SetTasksAction = { type: "set-tasks"; tasks: Task[] };

type MergeTasksAction = { type: "merge-tasks"; tasks: Task[] };

type ClearCommandsAction = { type: "clear-commands" };

type Action =
  | AddTaskAction
  | UpdateTaskAction
  | CompleteTaskAction
  | SetTasksAction
  | MergeTasksAction
  | ClearCommandsAction;
const categories: Task["category"][] = [
  "critical",
  "fun",
  "important",
  "normal",
];

function deriveCounters(tasks: Task[], prev: Counters): Counters {
  const next = { ...prev };
  for (const cat of categories) {
    const orders = tasks
      .filter((t) => t.category === cat)
      .map((t) => t.order ?? -1);
    const max = orders.length ? Math.max(...orders) + 1 : 0;
    if (max > next[cat]) next[cat] = max;
  }
  return next;
}

export function tasksReducer(state: State = initialState, action: Action): State {
  switch (action.type) {
    case "set-tasks":
      return {
        ...state,
        tasks: action.tasks,
        nextOrder: deriveCounters(action.tasks, state.nextOrder),
      };
    case "merge-tasks": {
      const merged = [...state.tasks];
      for (const t of action.tasks) {
        const idx = merged.findIndex((m) => m.id === t.id);
        if (idx >= 0) {
          merged[idx] = { ...merged[idx], ...t };
        } else {
          merged.push(t);
        }
      }
      return {
        ...state,
        tasks: merged,
        nextOrder: deriveCounters(merged, state.nextOrder),
      };
    }
    case "add-task": {
      const { taskId, commandId, partial } = action;
      const order = state.nextOrder[partial.category];
      const task: Task = { id: taskId, ...partial, order, done: false };
      const cmd: Command = {
        id: commandId,
        entityId: taskId,
        entityType: "task",
        type: "create-task",
        data: { ...partial, order },
      };
      return {
        tasks: [...state.tasks, task],
        commands: [...state.commands, cmd],
        nextOrder: {
          ...state.nextOrder,
          [partial.category]: order + 1,
        },
      };
    }
    case "update-task": {
      const { id, commandId, changes } = action;
      const tasks = state.tasks.map((t) => (t.id === id ? { ...t, ...changes } : t));
      const cmd: Command = {
        id: commandId,
        entityId: id,
        entityType: "task",
        type: "update-task",
        data: changes,
      };
      return { tasks, commands: [...state.commands, cmd], nextOrder: state.nextOrder };
    }
    case "complete-task": {
      const { id, commandId } = action;
      const tasks = state.tasks.map((t) => (t.id === id ? { ...t, done: true } : t));
      const cmd: Command = {
        id: commandId,
        entityId: id,
        entityType: "task",
        type: "complete-task",
      };
      return { tasks, commands: [...state.commands, cmd], nextOrder: state.nextOrder };
    }
    case "clear-commands":
      return { ...state, commands: [] };
    default:
      return state;
  }
}

export { initialState, State, Action };
