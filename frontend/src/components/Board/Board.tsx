import {
  DndContext,
  DragEndEvent,
  MouseSensor,
  TouchSensor,
  useSensor,
  useSensors,
} from '@dnd-kit/core';
import { SortableContext, horizontalListSortingStrategy } from '@dnd-kit/sortable';
import { useState } from 'react';
import Lane from '@components/Lane';
import TaskDetails from '@components/TaskDetails';
import type { Category, Task, Settings } from '@modules/types';
import { aria } from '.';

interface Props {
  tasks: Task[];
  settings: Settings;
  updateTask: (id: string, changes: Partial<Task>) => void;
  completeTask: (id: string) => void;
  reopenTask: (id: string) => void;
}

const categories: Category[] = ['critical', 'fun', 'important', 'normal'];

function getNextOrder(tasks: Task[]) {
  return tasks.reduce((max, task) => Math.max(max, task.order ?? 0), -1) + 1;
}

export function handleDragEnd(
  ev: DragEndEvent,
  tasks: Task[],
  updateTask: (id: string, changes: Partial<Task>) => void,
  completeTask: (id: string) => void,
) {
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

  if (activeTask.done) {
    if (toCat === 'done') {
      return;
    }
    const targetLane = tasks.filter((t) => t.category === toCat && !t.done);
    const order = getNextOrder(targetLane);
    updateTask(active.id as string, { category: toCat, order, done: false });
    return;
  }

  if (fromCat !== toCat) {
    // move to another lane; place at end if dropped on lane itself
    const targetLane = tasks.filter((t) => t.category === toCat && !t.done);
    const order = getNextOrder(targetLane);
    updateTask(active.id as string, { category: toCat, order });
    return;
  }
}

export default function Board({ tasks, settings, updateTask, completeTask, reopenTask }: Props) {
  const sensors = useSensors(
    useSensor(MouseSensor, { activationConstraint: { distance: 5 } }),
    useSensor(TouchSensor, { activationConstraint: { distance: 5 } })
  );
  const [expanded, setExpanded] = useState<Category | 'done' | null>(null);
  const [selected, setSelected] = useState<Task | null>(null);
  const onDragEnd = (ev: DragEndEvent) =>
    handleDragEnd(ev, tasks, updateTask, completeTask);

  const handleTaskMove = (task: Task, direction: 'up' | 'down') => {
    if (task.done) {
      return;
    }
    const laneTasks = tasks
      .filter((t) => t.category === task.category && !t.done)
      .sort((a, b) => (a.order ?? 0) - (b.order ?? 0));
    const currentIndex = laneTasks.findIndex((t) => t.id === task.id);
    if (currentIndex === -1) {
      return;
    }
    const offset = direction === 'up' ? -1 : 1;
    const targetIndex = currentIndex + offset;
    if (targetIndex < 0 || targetIndex >= laneTasks.length) {
      return;
    }

    const currentTask = laneTasks[currentIndex];
    const targetTask = laneTasks[targetIndex];
    const currentOrder = currentTask.order ?? currentIndex;
    const targetOrder = targetTask.order ?? targetIndex;

    updateTask(currentTask.id, { order: targetOrder });
    updateTask(targetTask.id, { order: currentOrder });
  };

  if (selected) {
    return <TaskDetails task={selected} onBack={() => setSelected(null)} />;
  }

  if (expanded) {
    const laneTasks = (expanded === 'done'
      ? tasks.filter((t) => t.done)
      : tasks.filter((t) => t.category === expanded && !t.done)
    ).sort((a, b) => (a.order ?? 0) - (b.order ?? 0));

    return (
      <DndContext onDragEnd={onDragEnd} sensors={sensors}>
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
              limit={settings.tasksPerCategory}
              expanded
              onTaskClick={setSelected}
              onTaskComplete={(task) =>
                expanded === 'done' ? reopenTask(task.id) : completeTask(task.id)
              }
              onTaskMove={expanded === 'done' ? undefined : (task, dir) => handleTaskMove(task, dir)}
            />
          </SortableContext>
        </div>
      </DndContext>
    );
  }

  return (
    <DndContext onDragEnd={onDragEnd} sensors={sensors}>
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
                limit={settings.tasksPerCategory}
                onExpand={() => setExpanded(cat)}
                onTaskClick={setSelected}
                onTaskComplete={(task) => completeTask(task.id)}
                onTaskMove={(task, dir) => handleTaskMove(task, dir)}
              />
            </SortableContext>
          );
        })}
        {settings.showDoneTasks && (
          <Lane
            category="done"
            tasks={tasks.filter((t) => t.done)}
            limit={settings.tasksPerCategory}
            onExpand={() => setExpanded('done')}
            onTaskClick={setSelected}
            onTaskComplete={(task) => reopenTask(task.id)}
          />
        )}
      </div>
    </DndContext>
  );
}