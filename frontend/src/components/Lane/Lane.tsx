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
  const { isMobile, isLarge } = useLayout();
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
    <section className={`${isMobile ? 'mb-2' : 'mb-4'} flex h-full flex-col`}>
      <h2 className={isMobile ? 'mx-1 mb-1' : 'mx-2 mb-2'}>
        <button
          type="button"
          onClick={onExpand}
          className={`flex w-full items-center rounded-md bg-gray-50 font-semibold text-gray-700 shadow-sm transition-colors hover:bg-gray-100 ${isMobile ? 'gap-1 px-2 py-1 text-xs' : 'gap-2 px-3 py-2 text-sm'}`}
        >
          <span className="h-2 w-2 rounded-full" style={{ backgroundColor: palette[category] }} />
          {titleMap[category]}
        </button>
      </h2>
      <div
        ref={setNodeRef}
        style={droppableStyle}
        className={`flex flex-1 flex-wrap transition-colors ${expanded ? 'overflow-auto' : 'overflow-hidden'} ${isMobile ? 'gap-1 px-1 pb-2 pt-2' : 'gap-2 px-2 pb-4 pt-4'}`}
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
            className={`flex items-center justify-center rounded-lg bg-gray-100 text-gray-500 shadow transition-colors hover:bg-gray-200 ${isMobile ? 'min-w-[60px] px-1 py-1 text-xs' : 'min-w-[160px] px-4 py-3 text-sm'}`}
          >
            +{extra} more
          </button>
        )}
      </div>
    </section>
  );
}
