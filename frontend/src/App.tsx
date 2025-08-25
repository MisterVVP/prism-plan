import { useState, useCallback } from 'react';
import { useAuth0 } from '@auth0/auth0-react';
import { v4 as uuid } from 'uuid';
import Board from './components/Board';
import TaskModal from './components/TaskModal';
import { useTasks, useLoginUser, useSettings } from './hooks';
import UserMenu from './components/UserMenu';
import SearchBar from './components/SearchBar';
import AddTaskButton from './components/AddTaskButton';
import { aria } from './aria';

export default function App() {
  const { tasks, addTask, updateTask, completeTask } = useTasks();
  const { settings, updateSettings } = useSettings();
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [search, setSearch] = useState('');
  const { loginWithRedirect, logout, isAuthenticated, user, getAccessTokenSilently } = useAuth0();
  const baseUrl =
    (import.meta.env.VITE_API_BASE_URL as string | undefined) ||
    `${window.location.origin}/api`;
  const audience = import.meta.env.VITE_AUTH0_AUDIENCE as string;
  useLoginUser();

  const handleLogout = useCallback(async () => {
    if (user?.sub) {
      try {
        const token = await getAccessTokenSilently({
          authorizationParams: {
            audience,
            scope: 'openid profile email offline_access',
          },
        });
        const command = {
          id: uuid(),
          entityId: user.sub,
          entityType: 'user',
          type: 'logout-user',
        };
        await fetch(`${baseUrl}/commands`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            Authorization: `Bearer ${token}`,
          },
          body: JSON.stringify([command]),
        });
      } catch (err) {
        console.error(err);
      }
    }
    logout({ logoutParams: { returnTo: window.location.origin } });
  }, [user?.sub, getAccessTokenSilently, audience, baseUrl, logout]);

  const handleOpenModal = useCallback(() => setIsModalOpen(true), []);
  const handleSearchChange = useCallback((value: string) => setSearch(value), []);

  const filteredTasks = tasks.filter((task) => {
    const q = search.toLowerCase();
    return (
      task.title.toLowerCase().includes(q) ||
      (task.notes ?? '').toLowerCase().includes(q)
    );
  });

  return (
    <div className="flex min-h-screen flex-col p-2 space-y-2 sm:p-4 sm:space-y-6 lg:space-y-8">
      <header
        {...aria.header}
        className="flex items-center justify-between gap-2 sm:gap-4"
      >
        <UserMenu
          isAuthenticated={isAuthenticated}
          userPicture={user?.picture}
          onLogin={loginWithRedirect}
          onLogout={handleLogout}
          settings={settings}
          onUpdateSettings={updateSettings}
        />
        <SearchBar value={search} onChange={handleSearchChange} />
        <AddTaskButton onAdd={handleOpenModal} />
      </header>

      <main {...aria.main} className="flex w-full flex-1 overflow-x-auto">
        <Board
          tasks={filteredTasks}
          settings={settings}
          updateTask={updateTask}
          completeTask={completeTask}
        />
      </main>

      <footer {...aria.footer} className="pt-2 text-center text-[10px] text-gray-500 sm:pt-4 sm:text-xs">
        Copyright © 2025 Vladimir Pavlov. All rights reserved.
      </footer>

      <TaskModal
        isOpen={isModalOpen}
        onClose={() => setIsModalOpen(false)}
        addTask={addTask}
      />
    </div>
  );
}
