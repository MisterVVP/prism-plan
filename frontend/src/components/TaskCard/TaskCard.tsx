import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { useRef } from 'react';
import type { Task } from '../../types';
import { palette } from '../../palette';

interface Props {
  task: Task;
  onClick?: () => void;
  onDoubleClick?: () => void;
}

export default function TaskCard({ task, onClick, onDoubleClick }: Props) {
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

  function handleClick(ev: React.MouseEvent<HTMLDivElement>) {
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
      onClick={handleClick}
      className="relative select-none rounded-lg border-l-4 bg-white text-gray-800 shadow transition-shadow touch-none hover:shadow-md cursor-pointer min-w-[60px] px-1 py-1 text-xs sm:min-w-[160px] sm:px-4 sm:py-3 sm:text-sm"
    >
      <div className="font-medium">{task.title}</div>
      {task.notes && (
        <div className="mt-1 text-gray-500 text-[10px] sm:text-xs whitespace-pre-line break-words overflow-hidden text-ellipsis task-card-note">
          {task.notes}
        </div>
      )}
    </div>
  );
}
