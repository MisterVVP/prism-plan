import { Fragment, useState, useEffect } from 'react';
import { Dialog, Transition, RadioGroup } from '@headlessui/react';
import { CheckIcon } from '@heroicons/react/20/solid';
import type { Category, Task } from '@modules/types';

interface Props {
  isOpen: boolean;
  onClose: () => void;
  addTask: (t: Omit<Task, 'id' | 'order' | 'done'>) => void;
  presetCategory?: Category;           // optional lane pre-select
}

const categories: { value: Category; label: string; bg: string }[] = [
  { value: 'critical',  label: 'Critical',  bg: 'bg-critical'  },
  { value: 'fun',       label: 'Fun',       bg: 'bg-fun'       },
  { value: 'important', label: 'Important', bg: 'bg-important' },
  { value: 'normal',    label: 'Normal',    bg: 'bg-normal'    }
];

function classNames(...classes: Array<string | false | null | undefined>) {
  return classes.filter(Boolean).join(' ');
}

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

  // reset selected category when preset changes or modal is reopened
  useEffect(() => {
    if (isOpen) setCat(presetCategory);
  }, [presetCategory, isOpen]);

  function handleSave() {
    addTask({ title: title.trim(), notes, category: cat });
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
                  className="mt-1 w-full rounded-md border-gray-300 p-2.5 text-base focus:border-indigo-500 focus:ring-indigo-500"
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
                  className="mt-1 w-full rounded-md border-gray-300 p-2.5 text-base focus:border-indigo-500 focus:ring-indigo-500"
                  value={notes}
                  onChange={e => setNotes(e.target.value)}
                  placeholder="Optional details or link"
                />
              </label>

              {/* Category picker */}
              <RadioGroup value={cat} onChange={setCat} className="mt-4">
                <RadioGroup.Label className="mb-1 block text-sm font-medium text-gray-700">
                  Category
                </RadioGroup.Label>
                <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
                  {categories.map(({ value, label, bg }) => (
                    <RadioGroup.Option
                      key={value}
                      value={value}
                      className={({ active, checked }) =>
                        classNames(
                          'flex cursor-pointer items-center justify-between rounded-lg border px-4 py-3 text-sm font-medium shadow-sm transition focus:outline-none',
                          checked ? 'border-indigo-500 bg-indigo-50 text-indigo-900' : 'border-gray-200 bg-white text-gray-700',
                          active ? 'ring-2 ring-indigo-200 ring-offset-1' : ''
                        )
                      }
                    >
                      {({ checked }) => (
                        <>
                          <div className="flex items-center gap-3">
                            <span className={`h-3 w-3 rounded-full ${bg}`} aria-hidden="true" />
                            <RadioGroup.Label as="span">{label}</RadioGroup.Label>
                          </div>
                          {checked ? (
                            <CheckIcon aria-hidden="true" className="h-5 w-5 text-indigo-500" />
                          ) : (
                            <span aria-hidden="true" className="h-5 w-5" />
                          )}
                        </>
                      )}
                    </RadioGroup.Option>
                  ))}
                </div>
              </RadioGroup>

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
