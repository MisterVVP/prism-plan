import TaskCard from './TaskCard';
import { useDroppable } from '@dnd-kit/core';
import type { Task, Category } from '../types';

const colorMap = {
  critical: '#FF5252',
  fun: '#4CAF50',
  important: '#3F7FBF',
  normal: '#D2D2D2',
  done: '#9CA3AF'
} as const;

interface Props {
  category: Category | 'done';
  tasks: Task[];
  onDone?: (id: string) => void;
}
export default function Lane({ category, tasks, onDone }: Props) {
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
        backgroundColor: `${colorMap[category]}20`,
        border: `2px dashed ${colorMap[category]}`,
        borderRadius: '0.5rem'
      }
    : undefined;
  return (
    <section className="mb-4 flex h-full flex-col ">
      <h2 className={`mx-2 mb-1 font-bold text-${category}`}>{titleMap[category]}</h2>
      <div
        ref={setNodeRef}
        style={droppableStyle}
        className="flex flex-1 gap-2 overflow-x-auto px-2 pb-4 sm:flex-wrap sm:overflow-visible transition-colors"
      >
        {tasks.map((task) => (
          <TaskCard key={task.id} task={task} onDone={onDone} />
        ))}
      </div>
    </section>
  );
}