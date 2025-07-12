import { Fragment, useState } from 'react';
import { Dialog, Transition } from '@headlessui/react';
import type { Category, Task } from '../types';

interface Props {
  isOpen: boolean;
  onClose: () => void;
  addTask: (t: Omit<Task, 'id'>) => void;
  presetCategory?: Category;           // optional lane pre-select
}

const categories: { value: Category; label: string; bg: string }[] = [
  { value: 'critical',  label: 'Critical',  bg: 'bg-critical'  },
  { value: 'fun',       label: 'Fun',       bg: 'bg-fun'       },
  { value: 'important', label: 'Important', bg: 'bg-important' },
  { value: 'normal',    label: 'Normal',    bg: 'bg-normal'    }
];

export default function TaskModal({
  isOpen,
  onClose,
  addTask,
  presetCategory = 'normal'
}: Props) {
  const [title, setTitle]       = useState('');
  const [notes, setNotes]       = useState('');
  const [cat, setCat]           = useState<Category>(presetCategory);
  const isSaveDisabled          = title.trim() === '';

  function handleSave() {
    addTask({ title: title.trim(), notes, category: cat, order: 0 });
    // reset & close
    setTitle('');
    setNotes('');
    setCat(presetCategory);
    onClose();
  }

  return (
    <Transition appear show={isOpen} as={Fragment}>
      <Dialog as="div" className="relative z-50" onClose={onClose}>
        <Transition.Child
          as={Fragment}
          enter="ease-out duration-200" enterFrom="opacity-0" enterTo="opacity-100"
          leave="ease-in duration-150" leaveFrom="opacity-100" leaveTo="opacity-0"
        >
          <div className="fixed inset-0 bg-black/30" />
        </Transition.Child>

        <div className="fixed inset-0 flex items-center justify-center p-4">
          <Transition.Child
            as={Fragment}
            enter="ease-out duration-200" enterFrom="scale-95 opacity-0" enterTo="scale-100 opacity-100"
            leave="ease-in duration-150" leaveFrom="scale-100 opacity-100" leaveTo="scale-95 opacity-0"
          >
            <Dialog.Panel className="w-full max-w-md rounded-xl bg-white p-6 shadow-xl">
              <Dialog.Title className="mb-4 text-lg font-bold">Add Task</Dialog.Title>

              {/* Title */}
              <label className="block text-sm font-medium text-gray-700">
                Title<span className="text-red-500">*</span>
                <input
                  type="text"
                  className="mt-1 w-full rounded-md border-gray-300 focus:border-indigo-500 focus:ring-indigo-500"
                  value={title}
                  onChange={e => setTitle(e.target.value)}
                  placeholder="Buy birthday gift…"
                  autoFocus
                />
              </label>

              {/* Notes */}
              <label className="mt-4 block text-sm font-medium text-gray-700">
                Notes
                <textarea
                  rows={3}
                  className="mt-1 w-full rounded-md border-gray-300 focus:border-indigo-500 focus:ring-indigo-500"
                  value={notes}
                  onChange={e => setNotes(e.target.value)}
                  placeholder="Optional details or link"
                />
              </label>

              {/* Category picker */}
              <fieldset className="mt-4">
                <legend className="mb-1 text-sm font-medium text-gray-700">Category</legend>
                <div className="flex flex-wrap gap-2">
                  {categories.map(({ value, label, bg }) => (
                    <button
                      key={value}
                      type="button"
                      onClick={() => setCat(value)}
                      className={`
                        ${bg} ${value === 'fun' ? 'rounded-full' : value === 'critical' ? 'clip-hex-tab' :
                        value === 'important' ? 'clip-bookmark-notch' : 'rounded-md'}
                        px-3 py-1 text-sm font-medium text-white shadow
                        ${cat === value ? 'ring-2 ring-offset-2 ring-indigo-500' : 'opacity-80 hover:opacity-100'}
                      `}
                    >
                      {label}
                    </button>
                  ))}
                </div>
              </fieldset>

              {/* Actions */}
              <div className="mt-6 flex justify-end gap-3">
                <button
                  onClick={onClose}
                  className="rounded-md bg-gray-100 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-200"
                >
                  Cancel
                </button>
                <button
                  disabled={isSaveDisabled}
                  onClick={handleSave}
                  className={`rounded-md px-4 py-2 text-sm font-medium text-white
                    ${isSaveDisabled ? 'bg-gray-400 cursor-not-allowed' : 'bg-indigo-600 hover:bg-indigo-700'}`}
                >
                  Save
                </button>
              </div>
            </Dialog.Panel>
          </Transition.Child>
        </div>
      </Dialog>
    </Transition>
  );
}
