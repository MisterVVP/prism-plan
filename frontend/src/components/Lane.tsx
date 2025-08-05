import TaskCard from './TaskCard';
import { useDroppable } from '@dnd-kit/core';
import type { Task, Category } from '../types';
import { palette } from '../palette';

interface Props {
  category: Category | 'done';
  tasks: Task[];
  onExpand?: () => void;
  expanded?: boolean;
}

export default function Lane({ category, tasks, onExpand, expanded }: Props) {
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

  const maxVisible = expanded
    ? tasks.length
    : typeof window !== 'undefined' && window.innerWidth >= 1024
      ? 6
      : 3;
  const visibleTasks = tasks.slice(0, maxVisible);
  const extra = tasks.length - maxVisible;

  return (
    <section className="mb-4 flex h-full flex-col">
      <h2 className="mx-2 mb-2">
        <button
          type="button"
          onClick={onExpand}
          className="flex w-full items-center gap-2 rounded-md bg-gray-50 px-3 py-2 text-sm font-semibold text-gray-700 shadow-sm transition-colors hover:bg-gray-100"
        >
          <span className="h-2 w-2 rounded-full" style={{ backgroundColor: palette[category] }} />
          {titleMap[category]}
        </button>
      </h2>
      <div
        ref={setNodeRef}
        style={droppableStyle}
        className={`flex flex-1 flex-wrap gap-2 px-2 pb-4 pt-4 transition-colors ${expanded ? 'overflow-auto' : 'overflow-hidden'}`}
      >
        {visibleTasks.map((task) => (
          <TaskCard key={task.id} task={task} />
        ))}
        {extra > 0 && (
          <button
            type="button"
            onClick={onExpand}
            className="flex min-w-[160px] items-center justify-center rounded-lg bg-gray-100 px-4 py-3 text-sm text-gray-500 shadow transition-colors hover:bg-gray-200"
          >
            +{extra} more
          </button>
        )}
      </div>
    </section>
  );
}
