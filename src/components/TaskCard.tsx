import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import type { Task } from '../types';

export default function TaskCard({ task }: { task: Task }) {
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
    critical: 'clip-hex',
    fun: 'rounded-full',
    important: 'clip-bookmark',
    normal: 'rounded-md'
  } as const;

  return (
    <div
      ref={setNodeRef}
      style={style}
      {...listeners}
      {...attributes}
      className={`min-w-[120px] px-3 py-2 text-sm text-white bg-${task.category} ${shapeClasses[task.category]} select-none shadow`}
    >
      {task.title}
    </div>
  );
}