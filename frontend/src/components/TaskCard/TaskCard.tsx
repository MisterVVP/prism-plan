import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import type { Task } from '../../types';
import { palette } from '../../palette';
import { useLayout } from '../../context/LayoutContext';

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
  const { isMobile, isLarge } = useLayout();

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    borderColor: palette[task.category]
  };

  const maxLines = isMobile ? 1 : isLarge ? 6 : 4;
  const noteStyle: React.CSSProperties = {
    WebkitLineClamp: maxLines,
    lineClamp: maxLines as any,
    display: '-webkit-box',
    WebkitBoxOrient: 'vertical' as any,
    overflow: 'hidden'
  };

  return (
    <div
      ref={setNodeRef}
      style={style}
      {...listeners}
      {...attributes}
      onClick={onClick}
      className={`relative select-none rounded-lg border-l-4 bg-white text-gray-800 shadow transition-shadow touch-none hover:shadow-md cursor-pointer ${isMobile ? 'min-w-[60px] px-1 py-1 text-xs' : 'min-w-[160px] px-4 py-3 text-sm'}`}
    >
      <div className="font-medium">{task.title}</div>
      {task.notes && (
        <div
          className={`mt-1 text-gray-500 ${isMobile ? 'text-[10px]' : 'text-xs'}`}
          style={noteStyle}
        >
          {task.notes}
        </div>
      )}
    </div>
  );
}
