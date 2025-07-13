import { useEffect, useState } from 'react';
import { useAuth0 } from '@auth0/auth0-react';
import { loadTasks, saveTasks } from '../storage';
import { v4 as uuid } from 'uuid';
import type { Task } from '../types';

export function useTasks() {
  const [tasks, setTasks] = useState<Task[]>([]);
  const { isAuthenticated } = useAuth0();
  const baseUrl = import.meta.env.VITE_API_BASE_URL as string;

  useEffect(() => {
    loadTasks().then(setTasks);
  }, []);

  useEffect(() => {
    if (!isAuthenticated) return;
    async function fetchRemote() {
      try {
        const res = await fetch(`${baseUrl}/tasks`);
        if (res.ok) {
          const data = await res.json();
          setTasks(data);
          saveTasks(data);
        }
      } catch (err) {
        console.error(err);
      }
    }
    fetchRemote();
  }, [isAuthenticated, baseUrl]);

  useEffect(() => {
    const id = setTimeout(() => {
      saveTasks(tasks);
      if (isAuthenticated) {
        fetch(`${baseUrl}/tasks`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(tasks),
        }).catch(() => {});
      }
    }, 500);
    return () => clearTimeout(id);
  }, [tasks, isAuthenticated, baseUrl]);

  function addTask(partial: Omit<Task, 'id'>) {
    setTasks((t) => [...t, { ...partial, id: uuid() }]);
  }

  function updateTask(id: string, changes: Partial<Task>) {
    setTasks((t) => t.map((task) => (task.id === id ? { ...task, ...changes } : task)));
  }

  return { tasks, addTask, updateTask };
}
