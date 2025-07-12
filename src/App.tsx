import { useState } from 'react';
import TaskLane from './components/Lane';
import { Task, TaskType } from './types';

const initialTasks: Task[] = [
  { id: '1', title: 'Pay taxes', type: 'critical' },
  { id: '2', title: 'Go jogging', type: 'fun' },
  { id: '3', title: 'Quarterly report', type: 'important' },
  { id: '4', title: 'Buy groceries', type: 'normal' },
];

export default function App() {
  const [tasks, setTasks] = useState<Task[]>(initialTasks);

  const moveTask = (id: string, to: TaskType) =>
    setTasks(tasks.map((t) => (t.id === id ? { ...t, type: to } : t)));

  const lanes: Record<TaskType, Task[]> = {
    critical: [],
    fun: [],
    important: [],
    normal: [],
  };
  tasks.forEach((t) => lanes[t.type].push(t));

  return (
    <div className="p-4 space-y-4">
      <header className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Task Groups</h1>
        <button className="rounded-lg border px-3 py-1 text-sm hover:bg-slate-100">
          + Add Task
        </button>
      </header>

      {(Object.keys(lanes) as TaskType[]).map((k) => (
        <TaskLane key={k} type={k} tasks={lanes[k]} onMove={moveTask} />
      ))}
    </div>
  );
}