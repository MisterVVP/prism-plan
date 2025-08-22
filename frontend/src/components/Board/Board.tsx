import {
  DndContext,
  DragEndEvent,
  MouseSensor,
  TouchSensor,
  useSensor,
  useSensors,
} from '@dnd-kit/core';
import { arrayMove, SortableContext, horizontalListSortingStrategy } from '@dnd-kit/sortable';
import Lane from '../Lane';
import TaskDetails from '../TaskDetails';
import type { Category, Task } from '../../types';
import { useState } from 'react';
import { aria } from './aria';

interface Props {
  tasks: Task[];
  updateTask: (id: string, changes: Partial<Task>) => void;
  completeTask: (id: string) => void;
}

const categories: Category[] = ['critical', 'fun', 'important', 'normal'];

export default function Board({ tasks, updateTask, completeTask }: Props) {
  const sensors = useSensors(
    useSensor(MouseSensor, { activationConstraint: { distance: 5 } }),
    useSensor(TouchSensor, { activationConstraint: { distance: 5 } })
  );
  const [expanded, setExpanded] = useState<Category | 'done' | null>(null);
  const [selected, setSelected] = useState<Task | null>(null);

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
      // move to another lane; place at end if dropped on lane itself
      const targetLane = tasks.filter((t) => t.category === toCat && !t.done);
      const newIndex = targetLane.findIndex((t) => t.id === over.id);
      const order = newIndex === -1 ? targetLane.length : newIndex;
      updateTask(active.id as string, { category: toCat, order });
      return;
    }

    // reorder within lane
    const laneTasks = tasks.filter((t) => t.category === fromCat && !t.done);
    const oldIndex = laneTasks.findIndex((t) => t.id === active.id);
    let newIndex = laneTasks.findIndex((t) => t.id === over.id);
    if (newIndex === -1) {
      newIndex = laneTasks.length - 1;
    }
    if (oldIndex === newIndex) {
      return;
    }

    const ordered = arrayMove(laneTasks, oldIndex, newIndex);
    ordered.forEach((task, idx) => updateTask(task.id, { order: idx }));
  }

  if (selected) {
    return <TaskDetails task={selected} onBack={() => setSelected(null)} />;
  }

  if (expanded) {
    const laneTasks = (expanded === 'done'
      ? tasks.filter((t) => t.done)
      : tasks.filter((t) => t.category === expanded && !t.done)
    ).sort((a, b) => (a.order ?? 0) - (b.order ?? 0));

    return (
      <DndContext onDragEnd={handleDragEnd} sensors={sensors}>
        <div {...aria.root} className="mx-auto flex max-w-5xl flex-col">
          <button
            type="button"
            className="mb-4 flex items-center gap-2 text-sm font-medium text-gray-600 hover:text-gray-800"
            onClick={() => setExpanded(null)}
          >
            ← Back to all categories
          </button>
          <SortableContext items={laneTasks.map((t) => t.id)} strategy={horizontalListSortingStrategy}>
            <Lane
              category={expanded}
              tasks={laneTasks}
              expanded
              onTaskClick={setSelected}
              onTaskComplete={(task) => completeTask(task.id)}
            />
          </SortableContext>
        </div>
      </DndContext>
    );
  }

  return (
    <DndContext onDragEnd={handleDragEnd} sensors={sensors}>
      <div {...aria.root} className="flex w-full flex-1 flex-col gap-2 sm:flex-row sm:flex-wrap sm:gap-4">
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
              <Lane
                category={cat}
                tasks={laneTasks}
                onExpand={() => setExpanded(cat)}
                onTaskClick={setSelected}
                onTaskComplete={(task) => completeTask(task.id)}
              />
            </SortableContext>
          );
        })}
        <Lane
          category="done"
          tasks={tasks.filter((t) => t.done)}
          onExpand={() => setExpanded('done')}
          onTaskClick={setSelected}
        />
      </div>
    </DndContext>
  );
}