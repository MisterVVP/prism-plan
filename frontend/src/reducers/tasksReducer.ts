import type { Task, Command } from "../types";

type State = {
  tasks: Task[];
  commands: Command[];
};

const initialState: State = { tasks: [], commands: [] };

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

type ClearCommandsAction = { type: "clear-commands" };

type Action =
  | AddTaskAction
  | UpdateTaskAction
  | CompleteTaskAction
  | SetTasksAction
  | ClearCommandsAction;

function computeNextOrder(state: State, category: Task["category"]): number {
  const existing = [
    ...state.tasks
      .filter((t) => t.category === category)
      .map((t) => t.order ?? -1),
    ...state.commands
      .filter(
        (c) =>
          c.type === "create-task" &&
          (c.data as any).category === category
      )
      .map((c) => ((c.data as any).order as number) ?? -1),
  ];
  return (existing.length ? Math.max(...existing) : -1) + 1;
}

export function tasksReducer(state: State = initialState, action: Action): State {
  switch (action.type) {
    case "set-tasks":
      return { ...state, tasks: action.tasks };
    case "add-task": {
      const { taskId, commandId, partial } = action;
      const order = computeNextOrder(state, partial.category);
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
      return { tasks, commands: [...state.commands, cmd] };
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
      return { tasks, commands: [...state.commands, cmd] };
    }
    case "clear-commands":
      return { ...state, commands: [] };
    default:
      return state;
  }
}

export { initialState, State, Action };
