import { DndContext, DragEndEvent } from '@dnd-kit/core';
import { arrayMove, SortableContext, horizontalListSortingStrategy } from '@dnd-kit/sortable';
import Lane from './Lane';
import type { Category, Task } from '../types';

interface Props {
  tasks: Task[];
  updateTask: (id: string, changes: Partial<Task>) => void;
  completeTask: (id: string) => void;
}

const categories: Category[] = ['critical', 'fun', 'important', 'normal'];

export default function Board({ tasks, updateTask, completeTask }: Props) {

  function handleDragEnd(ev: DragEndEvent) {
    const { active, over } = ev;
    if (!over) {
      return;
    }

    const toCat = over.data.current?.category as Category | 'done';
    const activeTask = tasks.find((t) => t.id === active.id);
    if (!activeTask) {
      return;
    }

    const fromCat = activeTask.category;

    if (toCat === 'done') {
      if (!activeTask.done) completeTask(active.id as string);
      return;
    }

    if (fromCat !== toCat) {
      updateTask(active.id as string, { category: toCat });
      return;
    }

    // reorder within lane
    const laneTasks = tasks.filter((t) => t.category === fromCat && !t.done);
    const oldIndex = laneTasks.findIndex((t) => t.id === active.id);
    const newIndex = laneTasks.findIndex((t) => t.id === over.id);
    if (oldIndex === newIndex || newIndex === -1) {
      return;
    }

    const ordered = arrayMove(laneTasks, oldIndex, newIndex);
    ordered.forEach((task, idx) => updateTask(task.id, { order: idx }));
  }

  return (
    <DndContext onDragEnd={handleDragEnd}>
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-5">
        {categories.map((cat) => {
          const laneTasks = tasks
            .filter((t) => t.category === cat && !t.done)
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
        <Lane category="done" tasks={tasks.filter((t) => t.done)} />
      </div>
    </DndContext>
  );
}