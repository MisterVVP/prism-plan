import { memo } from 'react';
import { PlusIcon } from '@heroicons/react/24/solid';
import { aria } from './aria';

interface AddTaskButtonProps {
  onAdd: () => void;
}

function AddTaskButton({ onAdd }: AddTaskButtonProps) {
  return (
    <div className="flex items-center">
      <button
        onClick={onAdd}
        {...aria.button}
        className="rounded-full bg-indigo-600 text-white hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-indigo-500 h-8 w-8 p-1 sm:h-10 sm:w-10 sm:p-2"
      >
        <PlusIcon className="h-full w-full" />
      </button>
    </div>
  );
}

export default memo(AddTaskButton);
