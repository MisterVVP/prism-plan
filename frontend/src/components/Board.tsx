import {
  DndContext,
  DragEndEvent,
  MouseSensor,
  TouchSensor,
  useSensor,
  useSensors,
} from '@dnd-kit/core';
import { arrayMove, SortableContext, horizontalListSortingStrategy } from '@dnd-kit/sortable';
import Lane from './Lane';
import type { Category, Task } from '../types';
import { useState } from 'react';

interface Props {
  tasks: Task[];
  updateTask: (id: string, changes: Partial<Task>) => void;
  completeTask: (id: string) => void;
}

const categories: Category[] = ['critical', 'fun', 'important', 'normal'];

export default function Board({ tasks, updateTask, completeTask }: Props) {
  const sensors = useSensors(useSensor(MouseSensor), useSensor(TouchSensor));
  const [expanded, setExpanded] = useState<Category | 'done' | null>(null);

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

  if (expanded) {
    const laneTasks = (expanded === 'done'
      ? tasks.filter((t) => t.done)
      : tasks.filter((t) => t.category === expanded && !t.done)
    ).sort((a, b) => (a.order ?? 0) - (b.order ?? 0));

    return (
      <DndContext onDragEnd={handleDragEnd} sensors={sensors}>
        <div className="mx-auto max-w-5xl">
          <button
            type="button"
            className="mb-4 flex items-center gap-2 text-sm font-medium text-gray-600 hover:text-gray-800"
            onClick={() => setExpanded(null)}
          >
            ← Back to all categories
          </button>
          <SortableContext items={laneTasks.map((t) => t.id)} strategy={horizontalListSortingStrategy}>
            <Lane category={expanded} tasks={laneTasks} expanded />
          </SortableContext>
        </div>
      </DndContext>
    );
  }

  return (
    <DndContext onDragEnd={handleDragEnd} sensors={sensors}>
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
              <Lane category={cat} tasks={laneTasks} onExpand={() => setExpanded(cat)} />
            </SortableContext>
          );
        })}
        <Lane category="done" tasks={tasks.filter((t) => t.done)} onExpand={() => setExpanded('done')} />
      </div>
    </DndContext>
  );
}