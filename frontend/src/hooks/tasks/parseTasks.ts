import type { Task } from "../../types";

export function parseTasks(payload: string): Task[] {
  try {
    const data = JSON.parse(payload);
    return Array.isArray(data) ? (data as Task[]) : [];
  } catch {
    return [];
  }
}

