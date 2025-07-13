import { DndContext, DragEndEvent } from '@dnd-kit/core';
import { arrayMove, SortableContext, horizontalListSortingStrategy } from '@dnd-kit/sortable';
import Lane from './Lane';
import type { Category, Task } from '../types';

interface Props {
  tasks: Task[];
  updateTask: (id: string, changes: Partial<Task>) => void;
}

const categories: Category[] = ['critical', 'fun', 'important', 'normal'];

export default function Board({ tasks, updateTask }: Props) {

  function handleDragEnd(ev: DragEndEvent) {
    const { active, over } = ev;
    if (!over) return;
    const fromCat = active.data.current?.category as Category;
    const toCat = over.data.current?.category as Category;

    if (fromCat !== toCat) {
      updateTask(active.id as string, { category: toCat });
      return;
    }
    // reorder within lane
    const laneTasks = tasks.filter((t) => t.category === fromCat);
    const oldIndex = laneTasks.findIndex((t) => t.id === active.id);
    const newIndex = laneTasks.findIndex((t) => t.id === over.id);
    const ordered = arrayMove(laneTasks, oldIndex, newIndex);
    ordered.forEach((task, idx) => updateTask(task.id, { order: idx }));
  }

  return (
    <DndContext onDragEnd={handleDragEnd}>
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {categories.map((cat) => {
          const laneTasks = tasks
            .filter((t) => t.category === cat)
            .sort((a, b) => (a.order ?? 0) - (b.order ?? 0));
          return (
            <SortableContext
              items={laneTasks.map((t) => t.id)}
              strategy={horizontalListSortingStrategy}
              key={cat}
            >
              <Lane category={cat} tasks={laneTasks} />
            </SortableContext>
          );
        })}
      </div>
    </DndContext>
  );
}