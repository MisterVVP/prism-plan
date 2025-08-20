import TaskCard from '../TaskCard';
import { useDroppable } from '@dnd-kit/core';
import type { Task, Category } from '../../types';
import { palette } from '../../palette';
import { useLayout } from '../../context/LayoutContext';

interface Props {
  category: Category | 'done';
  tasks: Task[];
  onExpand?: () => void;
  expanded?: boolean;
  onTaskClick?: (task: Task) => void;
  onTaskComplete?: (task: Task) => void;
}

export default function Lane({ category, tasks, onExpand, expanded, onTaskClick, onTaskComplete }: Props) {
  const { setNodeRef, isOver } = useDroppable({ id: category, data: { category } });
  const { isLarge } = useLayout();
  const titleMap = {
    critical: 'Critical',
    fun: 'Fun',
    important: 'Important',
    normal: 'Normal',
    done: 'Done'
  };
  const droppableStyle: React.CSSProperties | undefined = isOver
    ? {
        backgroundColor: `${palette[category]}20`,
        border: `2px dashed ${palette[category]}`,
        borderRadius: '0.5rem'
      }
    : undefined;

  const maxVisible = expanded ? tasks.length : isLarge ? 6 : 3;
  const visibleTasks = tasks.slice(0, maxVisible);
  const extra = tasks.length - maxVisible;

  return (
    <section className="mb-2 flex h-full flex-col sm:mb-4">
      <h2 className="mx-1 mb-1 sm:mx-2 sm:mb-2">
        <button
          type="button"
          onClick={onExpand}
          className="flex w-full items-center gap-1 rounded-md bg-gray-50 px-2 py-1 text-xs font-semibold text-gray-700 shadow-sm transition-colors hover:bg-gray-100 sm:gap-2 sm:px-3 sm:py-2 sm:text-sm"
        >
          <span className="h-2 w-2 rounded-full" style={{ backgroundColor: palette[category] }} />
          {titleMap[category]}
        </button>
      </h2>
      <div
        ref={setNodeRef}
        style={droppableStyle}
        className={`flex flex-1 flex-col transition-colors ${expanded ? 'overflow-auto' : 'overflow-hidden'} gap-1 px-1 pb-2 pt-2 sm:gap-2 sm:px-2 sm:pb-4 sm:pt-4`}
      >
        {visibleTasks.map((task) => (
          <TaskCard
            key={task.id}
            task={task}
            onClick={() => onTaskClick?.(task)}
            onDoubleClick={() => onTaskComplete?.(task)}
          />
        ))}
        {extra > 0 && (
          <button
            type="button"
            onClick={onExpand}
            className="flex items-center justify-center rounded-lg bg-gray-100 px-1 py-1 text-xs text-gray-500 shadow transition-colors hover:bg-gray-200 min-w-[60px] sm:min-w-[160px] sm:px-4 sm:py-3 sm:text-sm"
          >
            +{extra} more
          </button>
        )}
      </div>
    </section>
  );
}
