import { useEffect, useState } from 'react';
import { loadTasks, saveTasks } from '../storage';
import { v4 as uuid } from 'uuid';
import type { Task } from '../types';

export function useTasks() {
  const [tasks, setTasks] = useState<Task[]>([]);

  useEffect(() => {
    loadTasks().then(setTasks);
  }, []);

  useEffect(() => {
    const id = setTimeout(() => saveTasks(tasks), 500);
    return () => clearTimeout(id);
  }, [tasks]);

  function addTask(partial: Omit<Task, 'id'>) {
    setTasks((t) => [...t, { ...partial, id: uuid() }]);
  }

  function updateTask(id: string, changes: Partial<Task>) {
    setTasks((t) => t.map((task) => (task.id === id ? { ...task, ...changes } : task)));
  }

  return { tasks, addTask, updateTask };
}