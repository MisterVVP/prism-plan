import { useState } from 'react';
import Board from './components/Board';
import TaskModal from './components/TaskModal';
import { useTasks } from './hooks/useTasks';

export default function App() {
  const { tasks, addTask, updateTask } = useTasks();
  const [isModalOpen, setIsModalOpen] = useState(false);

  return (
    <div className="p-4 space-y-4">
      <header className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Task Groups</h1>
        <button
          className="rounded-lg border px-3 py-1 text-sm hover:bg-slate-100"
          onClick={() => setIsModalOpen(true)}
        >
          + Add Task
        </button>
      </header>

      <Board tasks={tasks} updateTask={updateTask} />

      <TaskModal
        isOpen={isModalOpen}
        onClose={() => setIsModalOpen(false)}
        addTask={addTask}
      />
    </div>
  );
}
