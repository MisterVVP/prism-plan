import { get, set } from 'idb-keyval';
import type { Task, TaskEvent } from './types';

const EVENT_KEY = 'events-v1';
const TASK_KEY = 'tasks-v1';

export async function loadEvents(): Promise<TaskEvent[]> {
  return (await get(EVENT_KEY)) ?? [];
}
export async function saveEvents(events: TaskEvent[]) {
  await set(EVENT_KEY, events);
}

export async function loadTasks(): Promise<Task[]> {
  return (await get(TASK_KEY)) ?? [];
}
export async function saveTasks(tasks: Task[]) {
  await set(TASK_KEY, tasks);
}