import { Fragment, memo } from 'react';
import { Menu, Transition } from '@headlessui/react';
import { UserCircleIcon } from '@heroicons/react/24/solid';
import { aria } from './aria';

interface UserMenuProps {
  isAuthenticated: boolean;
  userPicture?: string;
  onLogin: () => void;
  onLogout: () => void;
}

function UserMenu({ isAuthenticated, userPicture, onLogin, onLogout }: UserMenuProps) {
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
            <Menu.Items className="absolute left-0 mt-2 w-24 origin-top-left rounded-md bg-white shadow-lg ring-1 ring-black ring-opacity-5 focus:outline-none">
              <Menu.Item>
                {({ active }) => (
                  <button
                    onClick={onLogout}
                    className={`${active ? 'bg-gray-100' : ''} block w-full px-2 py-1 text-sm text-left`}
                  >
                    Log out
                  </button>
                )}
              </Menu.Item>
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
