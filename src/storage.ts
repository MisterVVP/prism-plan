import { get, set } from 'idb-keyval';
import type { Task } from './types';

const KEY = 'tasks-v1';
export async function loadTasks(): Promise<Task[]> {
  return (await get(KEY)) ?? [];
}
export async function saveTasks(tasks: Task[]) {
  await set(KEY, tasks);
}