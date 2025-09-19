import type { Task } from '@modules/types';

export function parseTasks(payload: string): Task[] {
  try {
    const msg = JSON.parse(payload);
    return msg.entityType === "task" && Array.isArray(msg.data)
      ? (msg.data as Task[])
      : [];
  } catch {
    return [];
  }
}

