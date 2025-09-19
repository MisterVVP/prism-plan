import { Fragment, memo, useCallback, useEffect, useRef, useState } from 'react';
import { Menu, Transition } from '@headlessui/react';
import { UserCircleIcon } from '@heroicons/react/24/solid';
import { aria } from '.';
import type { Settings } from '../../types';

interface UserMenuProps {
  isAuthenticated: boolean;
  userPicture?: string;
  onLogin: () => void;
  onLogout: () => void;
  settings?: Settings;
  onUpdateSettings?: (changes: Partial<Settings>) => void;
}

const TASKS_PER_CATEGORY_DEBOUNCE_MS = 400;

function UserMenu({ isAuthenticated, userPicture, onLogin, onLogout, settings, onUpdateSettings }: UserMenuProps) {
  const [tasksPerCategoryInput, setTasksPerCategoryInput] = useState(
    () => `${settings?.tasksPerCategory ?? 3}`
  );
  const updateTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const pendingTasksPerCategoryRef = useRef<number | null>(null);
  const updateSettingsRef = useRef(onUpdateSettings);

  useEffect(() => {
    updateSettingsRef.current = onUpdateSettings;
  }, [onUpdateSettings]);

  const currentTasksPerCategory = settings?.tasksPerCategory ?? 3;

  useEffect(() => {
    setTasksPerCategoryInput((current) => {
      const parsed = Number.parseInt(current, 10);
      const normalized = Number.isNaN(parsed) ? currentTasksPerCategory : parsed;
      if (normalized === currentTasksPerCategory) {
        return `${currentTasksPerCategory}`;
      }
      return current;
    });
  }, [currentTasksPerCategory]);

  useEffect(() => () => {
    if (updateTimeoutRef.current) {
      clearTimeout(updateTimeoutRef.current);
    }
  }, []);

  const scheduleTasksPerCategoryUpdate = useCallback((next: number) => {
    pendingTasksPerCategoryRef.current = next;
    if (!updateSettingsRef.current) {
      return;
    }
    if (updateTimeoutRef.current) {
      clearTimeout(updateTimeoutRef.current);
    }
    updateTimeoutRef.current = setTimeout(() => {
      updateTimeoutRef.current = null;
      const value = pendingTasksPerCategoryRef.current;
      if (typeof value === 'number') {
        pendingTasksPerCategoryRef.current = null;
        updateSettingsRef.current?.({ tasksPerCategory: value });
      }
    }, TASKS_PER_CATEGORY_DEBOUNCE_MS);
  }, []);

  const flushPendingTasksPerCategoryUpdate = useCallback(() => {
    if (updateTimeoutRef.current) {
      clearTimeout(updateTimeoutRef.current);
      updateTimeoutRef.current = null;
    }
    const value = pendingTasksPerCategoryRef.current;
    if (typeof value === 'number') {
      pendingTasksPerCategoryRef.current = null;
      updateSettingsRef.current?.({ tasksPerCategory: value });
    }
  }, []);

  const handleTasksPerCategoryChange = useCallback(
    (value: string) => {
      setTasksPerCategoryInput(value);
      const parsed = Number.parseInt(value, 10);
      const normalized = Number.isNaN(parsed) ? 0 : parsed;
      if (normalized === currentTasksPerCategory) {
        if (updateTimeoutRef.current) {
          clearTimeout(updateTimeoutRef.current);
          updateTimeoutRef.current = null;
        }
        pendingTasksPerCategoryRef.current = null;
        return;
      }
      scheduleTasksPerCategoryUpdate(normalized);
    },
    [currentTasksPerCategory, scheduleTasksPerCategoryUpdate]
  );

  return (
    <div className="flex items-center">
      {isAuthenticated ? (
        <Menu as="div" className="flex">
          <Menu.Button className="focus:outline-none">
            <img src={userPicture} {...aria.avatar} className="h-10 w-10 rounded-full" />
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
            <Menu.Items className="absolute left-0 mt-2 w-48 origin-top-left rounded-md bg-white p-2 shadow-lg ring-1 ring-black ring-opacity-5 focus:outline-none">
              <div className="flex flex-col gap-2">
                <label className="text-xs">
                  Tasks per category
                  <input
                    type="number"
                    min={1}
                    value={tasksPerCategoryInput}
                    onChange={(e) => handleTasksPerCategoryChange(e.target.value)}
                    onBlur={flushPendingTasksPerCategoryUpdate}
                    className="mt-1 w-full rounded border border-gray-300 px-1 py-0.5 text-xs"
                  />
                </label>
                <label className="flex items-center gap-1 text-xs">
                  <input
                    type="checkbox"
                    checked={settings?.showDoneTasks ?? false}
                    onChange={(e) => onUpdateSettings?.({ showDoneTasks: e.target.checked })}
                  />
                  Display done tasks
                </label>
                <Menu.Item>
                  {({ active }) => (
                    <button
                      onClick={onLogout}
                      className={`${active ? 'bg-gray-100' : ''} block w-full px-2 py-1 text-left text-sm`}
                    >
                      Log out
                    </button>
                  )}
                </Menu.Item>
              </div>
            </Menu.Items>
          </Transition>
        </Menu>
      ) : (
        <button onClick={onLogin} {...aria.loginButton} className="focus:outline-none">
          <UserCircleIcon className="h-8 w-8 cursor-pointer text-gray-400 sm:h-10 sm:w-10" />
        </button>
      )}
    </div>
  );
}

export default memo(UserMenu);
