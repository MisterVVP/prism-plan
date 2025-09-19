import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { useRef } from 'react';
import type { MouseEvent } from 'react';
import type { Task } from '../../types';
import { palette } from '../../palette';
import { aria } from '.';

interface Props {
  task: Task;
  onClick?: () => void;
  onDoubleClick?: () => void;
  onMoveUp?: () => void;
  onMoveDown?: () => void;
  showOrderControls?: boolean;
}

export default function TaskCard({
  task,
  onClick,
  onDoubleClick,
  onMoveUp,
  onMoveDown,
  showOrderControls,
}: Props) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition
  } = useSortable({
    id: task.id,
    data: { category: task.category }
  });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    borderColor: palette[task.category]
  };

  const clickTimeout = useRef<NodeJS.Timeout | null>(null);

  const handleMoveUp = (ev: MouseEvent<HTMLButtonElement>) => {
    ev.stopPropagation();
    ev.preventDefault();
    onMoveUp?.();
  };

  const handleMoveDown = (ev: MouseEvent<HTMLButtonElement>) => {
    ev.stopPropagation();
    ev.preventDefault();
    onMoveDown?.();
  };

  function handleClick(ev: MouseEvent<HTMLDivElement>) {
    if (ev.detail === 2) {
      if (clickTimeout.current) {
        clearTimeout(clickTimeout.current);
        clickTimeout.current = null;
      }
      onDoubleClick?.();
    } else if (ev.detail === 1) {
      if (clickTimeout.current) {
        clearTimeout(clickTimeout.current);
      }
      clickTimeout.current = setTimeout(() => {
        clickTimeout.current = null;
        onClick?.();
      }, 250);
    }
  }

  return (
    <div
      ref={setNodeRef}
      style={style}
      {...listeners}
      {...attributes}
      {...aria.root(task.title)}
      onClick={handleClick}
      className="w-full select-none rounded-lg border-l-4 bg-white text-gray-800 shadow transition-shadow touch-none hover:shadow-md cursor-pointer px-1 py-1 text-xs sm:px-4 sm:py-3 sm:text-sm flex items-start justify-between gap-2"
    >
      <div className="min-w-0 flex-1">
        <div className="font-medium break-words">{task.title}</div>
        {task.notes && (
          <div className="mt-1 whitespace-pre-line break-words text-[10px] text-gray-500 sm:text-xs overflow-hidden text-ellipsis task-card-note">
            {task.notes}
          </div>
        )}
      </div>
      {showOrderControls && (
        <div className="ml-1 flex flex-col items-center gap-1 sm:ml-2">
          <button
            type="button"
            onClick={handleMoveUp}
            disabled={!onMoveUp}
            aria-label={`Move ${task.title} up`}
            className="flex h-8 w-8 items-center justify-center rounded-md border border-transparent bg-gray-100 text-lg text-gray-600 transition hover:bg-gray-200 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-500 disabled:cursor-not-allowed disabled:opacity-40 sm:h-7 sm:w-7"
          >
            <span aria-hidden="true">↑</span>
          </button>
          <button
            type="button"
            onClick={handleMoveDown}
            disabled={!onMoveDown}
            aria-label={`Move ${task.title} down`}
            className="flex h-8 w-8 items-center justify-center rounded-md border border-transparent bg-gray-100 text-lg text-gray-600 transition hover:bg-gray-200 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-500 disabled:cursor-not-allowed disabled:opacity-40 sm:h-7 sm:w-7"
          >
            <span aria-hidden="true">↓</span>
          </button>
        </div>
      )}
    </div>
  );
}
