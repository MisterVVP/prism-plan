import { useState } from 'react';
import { useAuth0 } from '@auth0/auth0-react';
import Board from './components/Board';
import TaskModal from './components/TaskModal';
import { useTasks } from './hooks/useTasks';

export default function App() {
  const { tasks, addTask, updateTask } = useTasks();
  const [isModalOpen, setIsModalOpen] = useState(false);
  const { loginWithRedirect, logout, isAuthenticated } = useAuth0();

  return (
    <div className="p-4 space-y-4">
      <header className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Task Groups</h1>
        <div className="flex items-center space-x-2">
          {isAuthenticated ? (
            <button
              className="rounded-lg border px-3 py-1 text-sm hover:bg-slate-100"
              onClick={() =>
                logout({ logoutParams: { returnTo: window.location.origin } })
              }
            >
              Log out
            </button>
          ) : (
            <button
              className="rounded-lg border px-3 py-1 text-sm hover:bg-slate-100"
              onClick={() => loginWithRedirect()}
            >
              Log in
            </button>
          )}
          <button
            className="rounded-lg border px-3 py-1 text-sm hover:bg-slate-100"
            onClick={() => setIsModalOpen(true)}
          >
            + Add Task
          </button>
        </div>
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
