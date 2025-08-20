import { useState, Fragment } from 'react';
import { useAuth0 } from '@auth0/auth0-react';
import { Menu, Transition } from '@headlessui/react';
import { UserCircleIcon, PlusIcon } from '@heroicons/react/24/solid';
import { v4 as uuid } from 'uuid';
import Board from './components/Board';
import TaskModal from './components/TaskModal';
import { useTasks, useLoginUser } from './hooks';
import { useLayout } from './context/LayoutContext';

export default function App() {
  const { tasks, addTask, updateTask, completeTask } = useTasks();
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [search, setSearch] = useState('');
  const { loginWithRedirect, logout, isAuthenticated, user, getAccessTokenSilently } = useAuth0();
  const baseUrl =
    (import.meta.env.VITE_API_BASE_URL as string | undefined) ||
    `${window.location.origin}/api`;
  const audience = import.meta.env.VITE_AUTH0_AUDIENCE as string;
  useLoginUser();
  const { isMobile } = useLayout();

  async function handleLogout() {
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
  }

  const filteredTasks = tasks.filter((task) => {
    const q = search.toLowerCase();
    return (
      task.title.toLowerCase().includes(q) ||
      (task.notes ?? '').toLowerCase().includes(q)
    );
  });

  return (
    <div
      className={`flex min-h-screen flex-col ${isMobile ? 'p-2 space-y-2' : 'p-4 space-y-6 sm:space-y-8'}`}
    >
      <header className={`flex items-center justify-between ${isMobile ? 'gap-2' : 'gap-4'}`}>
        {/* User avatar / login */}
        <div className="flex items-center">
          {isAuthenticated ? (
            <Menu as="div" className="flex">
              <Menu.Button className="focus:outline-none">
                <img
                  src={user?.picture}
                  alt="User avatar"
                  className="h-10 w-10 rounded-full"
                />
              </Menu.Button>
            <Transition
              as={Fragment}
              enter="transition ease-out duration-100"
              enterFrom="transform opacity-0 scale-95"
              enterTo="transform opacity-100 scale-100"
              leave="transition ease-in duration-75"
              leaveFrom="transform opacity-100 scale-100"
              leaveTo="transform opacity-0 scale-95"
            >
              <Menu.Items className="absolute left-0 mt-2 w-24 origin-top-left rounded-md bg-white shadow-lg ring-1 ring-black ring-opacity-5 focus:outline-none">
                <Menu.Item>
                  {({ active }) => (
                    <button
                      onClick={() =>
                        handleLogout()
                      }
                      className={`${
                        active ? 'bg-gray-100' : ''
                      } block w-full px-2 py-1 text-sm text-left`}
                    >
                      Log out
                    </button>
                  )}
                </Menu.Item>
              </Menu.Items>
            </Transition>
          </Menu>
          ) : (
              <UserCircleIcon
                onClick={() => loginWithRedirect()}
                className={`${isMobile ? 'h-8 w-8' : 'h-10 w-10'} text-gray-400 cursor-pointer`}
              />
          )}
        </div>

        {/* Search bar */}
        <div className={`flex-1 ${isMobile ? 'px-1' : 'px-2 sm:px-4'}`}>
          <input
            type="text"
            placeholder="Search..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className={`w-full rounded-md border border-gray-300 focus:border-indigo-500 focus:ring-indigo-500 ${isMobile ? 'px-1 py-1 text-xs' : 'px-2 py-1 text-sm'}`}
          />
        </div>

        {/* Add task */}
        <div className="flex items-center">
          <button
            onClick={() => setIsModalOpen(true)}
            className={`rounded-full bg-indigo-600 text-white hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-indigo-500 ${isMobile ? 'h-8 w-8 p-1' : 'h-10 w-10 p-2'}`}
          >
            <PlusIcon className="h-full w-full" />
          </button>
        </div>
      </header>

      <main className="flex flex-1">
        <Board tasks={filteredTasks} updateTask={updateTask} completeTask={completeTask} />
      </main>

      <footer className={`${isMobile ? 'pt-2 text-[10px]' : 'pt-4 text-xs'} text-center text-gray-500`}>
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
