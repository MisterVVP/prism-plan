import { useState, Fragment } from 'react';
import { useAuth0 } from '@auth0/auth0-react';
import { Menu, Transition } from '@headlessui/react';
import { UserCircleIcon, PlusIcon } from '@heroicons/react/24/solid';
import Board from './components/Board';
import TaskModal from './components/TaskModal';
import { useTasks } from './hooks/useTasks';
import { useRegisterUser } from './hooks/useRegisterUser';

export default function App() {
  const { tasks, addTask, updateTask, completeTask } = useTasks();
  const [isModalOpen, setIsModalOpen] = useState(false);
  const { loginWithRedirect, logout, isAuthenticated, user } = useAuth0();
  useRegisterUser();

  return (
    <div className="p-4 space-y-6 sm:space-y-8">
      <header className="flex items-center justify-between gap-4">
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
                        logout({ logoutParams: { returnTo: window.location.origin } })
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
              className="h-10 w-10 text-gray-400 cursor-pointer"
            />
          )}
        </div>

        {/* Search bar */}
        <div className="flex-1 px-2 sm:px-4">
          <input
            type="text"
            placeholder="Search..."
            className="w-full rounded-md border border-gray-300 px-2 py-1 text-sm focus:border-indigo-500 focus:ring-indigo-500"
          />
        </div>

        {/* Add task */}
        <div className="flex items-center">
          <button
            onClick={() => setIsModalOpen(true)}
            className="h-10 w-10 rounded-full bg-indigo-600 p-2 text-white hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-indigo-500"
          >
            <PlusIcon className="h-full w-full" />
          </button>
        </div>
      </header>

      <Board tasks={tasks} updateTask={updateTask} completeTask={completeTask} />

      <footer className="pt-4 text-center text-xs text-gray-500">
        © 2025 Vladimir Pavlov. All rights reserved.
      </footer>

      <TaskModal
        isOpen={isModalOpen}
        onClose={() => setIsModalOpen(false)}
        addTask={addTask}
      />
    </div>
  );
}
