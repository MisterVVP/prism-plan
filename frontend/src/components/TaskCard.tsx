import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import type { Task } from '../types';
import { palette } from '../palette';

interface Props {
  task: Task;
  onClick?: () => void;
}

export default function TaskCard({ task, onClick }: Props) {
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

  const maxLines =
    typeof window !== 'undefined' && window.innerWidth >= 1024 ? 6 : 4;
  const clampClass = maxLines === 6 ? 'line-clamp-6' : 'line-clamp-4';

  return (
    <div
      ref={setNodeRef}
      style={style}
      {...listeners}
      {...attributes}
      onClick={onClick}
      className="relative min-w-[160px] select-none rounded-lg border-l-4 bg-white px-4 py-3 text-sm text-gray-800 shadow transition-shadow touch-none hover:shadow-md cursor-pointer"
    >
      <div className="font-medium">{task.title}</div>
      {task.notes && (
        <div className={`mt-1 text-xs text-gray-500 ${clampClass}`}>{task.notes}</div>
      )}
    </div>
  );
}