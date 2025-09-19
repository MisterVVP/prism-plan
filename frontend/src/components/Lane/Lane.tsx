import { useDroppable } from '@dnd-kit/core';
import TaskCard from '@components/TaskCard';
import type { Task, Category } from '@modules/types';
import { palette } from '@modules/palette';
import { aria } from '.';

interface Props {
  category: Category | 'done';
  tasks: Task[];
  limit: number;
  onExpand?: () => void;
  expanded?: boolean;
  onTaskClick?: (task: Task) => void;
  onTaskComplete?: (task: Task) => void;
  onTaskMove?: (task: Task, direction: 'up' | 'down') => void;
}

export default function Lane({
  category,
  tasks,
  limit,
  onExpand,
  expanded,
  onTaskClick,
  onTaskComplete,
  onTaskMove,
}: Props) {
  const { setNodeRef, isOver } = useDroppable({ id: category, data: { category } });
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

  const maxVisible = expanded ? tasks.length : limit;
  const visibleTasks = tasks.slice(0, maxVisible);
  const extra = tasks.length - maxVisible;
  const allowReorder = category !== 'done' && typeof onTaskMove === 'function';

  return (
    <section {...aria.section(category)} className="mb-2 flex w-full flex-col sm:mb-4 sm:flex-1 sm:min-w-[16rem]">
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
        className={`flex w-full flex-1 flex-col transition-colors ${expanded ? 'overflow-auto' : 'overflow-hidden'} gap-1 px-1 pb-2 pt-2 sm:gap-2 sm:px-2 sm:pb-4 sm:pt-4`}
      >
        {visibleTasks.map((task) => {
          const position = tasks.findIndex((t) => t.id === task.id);
          const canMoveUp = allowReorder && position > 0;
          const canMoveDown = allowReorder && position >= 0 && position < tasks.length - 1;
          return (
            <TaskCard
              key={task.id}
              task={task}
              onClick={() => onTaskClick?.(task)}
              onDoubleClick={() => onTaskComplete?.(task)}
              onMoveUp={canMoveUp ? () => onTaskMove?.(task, 'up') : undefined}
              onMoveDown={canMoveDown ? () => onTaskMove?.(task, 'down') : undefined}
              showOrderControls={allowReorder}
            />
          );
        })}
        {extra > 0 && (
          <button
            type="button"
            onClick={onExpand}
            className="flex w-full items-center justify-center rounded-lg bg-gray-100 px-1 py-1 text-xs text-gray-500 shadow transition-colors hover:bg-gray-200 sm:px-4 sm:py-3 sm:text-sm"
          >
            +{extra} more
          </button>
        )}
      </div>
    </section>
  );
}
