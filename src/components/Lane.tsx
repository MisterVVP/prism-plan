import TaskCard from './TaskCard';
import type { Task, Category } from '../types';

interface Props {
  category: Category;
  tasks: Task[];
}
export default function Lane({ category, tasks }: Props) {
  const titleMap = {
    critical: 'Critical',
    fun: 'Fun',
    important: 'Important',
    normal: 'Normal'
  };
  return (
    <section className="mb-4">
      <h2 className={`mx-2 mb-1 font-bold text-${category}`}>{titleMap[category]}</h2>
      <div className="flex gap-2 overflow-x-auto px-2 pb-4">
        {tasks.map((task) => (
          <TaskCard key={task.id} task={task} />
        ))}
      </div>
    </section>
  );
}