import { useState } from 'react';
import TaskLane from './components/Lane';
import type { Task, Category } from './types';

const initialTasks: Task[] = [
  { id: '1', title: 'Pay taxes', category: 'critical' },
  { id: '2', title: 'Go jogging', category: 'fun' },
  { id: '3', title: 'Quarterly report', category: 'important' },
  { id: '4', title: 'Buy groceries', category: 'normal' },
];

export default function App() {
  const [tasks, setTasks] = useState<Task[]>(initialTasks);


  const lanes: Record<Category, Task[]> = {
    critical: [],
    fun: [],
    important: [],
    normal: [],
  };
  tasks.forEach((t) => lanes[t.category].push(t));

  return (
    <div className="p-4 space-y-4">
      <header className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Task Groups</h1>
        <button className="rounded-lg border px-3 py-1 text-sm hover:bg-slate-100">
          + Add Task
        </button>
      </header>

      {(Object.keys(lanes) as Category[]).map((k) => (
        <TaskLane key={k} category={k} tasks={lanes[k]} />
      ))}
    </div>
  );
}