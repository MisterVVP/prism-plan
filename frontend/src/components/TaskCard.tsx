import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import type { Task } from '../types';

export default function TaskCard({ task, onDone }: { task: Task; onDone?: (id: string) => void }) {
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
    transition
  };

  const shapeClasses = {
    critical: 'clip-hex-tab',
    fun: 'rounded-full',
    important: 'clip-bookmark-notch',
    normal: 'rounded-md'
  } as const;

  return (
    <div
      ref={setNodeRef}
      style={style}
      {...listeners}
      {...attributes}
      className={`relative min-w-[120px] px-3 py-2 text-sm text-white bg-${task.category} ${shapeClasses[task.category]} select-none shadow`}
    >
      {task.title}
      {onDone && !task.done && (
        <button
          onClick={() => onDone(task.id)}
          className="absolute top-0 right-0 m-1 rounded bg-white/30 px-1 text-xs"
        >
          ✓
        </button>
      )}
    </div>
  );
}