import { get, set } from 'idb-keyval';
import type { Task, TaskEvent } from './types';

const EVENT_KEY = 'events-v1';
const TASK_KEY = 'tasks-v1';

function key(base: string, userId?: string | null) {
  return userId ? `${base}-${userId}` : `${base}-guest`;
}

export async function loadEvents(userId?: string | null): Promise<TaskEvent[]> {
  return (await get(key(EVENT_KEY, userId))) ?? [];
}
export async function saveEvents(userId: string | null | undefined, events: TaskEvent[]) {
  await set(key(EVENT_KEY, userId), events);
}

export async function loadTasks(userId?: string | null): Promise<Task[]> {
  return (await get(key(TASK_KEY, userId))) ?? [];
}
export async function saveTasks(userId: string | null | undefined, tasks: Task[]) {
  await set(key(TASK_KEY, userId), tasks);
}