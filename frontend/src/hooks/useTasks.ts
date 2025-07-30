import { useEffect, useState } from 'react';
import { useAuth0 } from '@auth0/auth0-react';
import { v4 as uuid } from 'uuid';
import {
  loadEvents,
  saveEvents,
  loadTasks,
  saveTasks
} from '../storage';
import type { Task, TaskEvent } from '../types';

export function useTasks() {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [events, setEvents] = useState<TaskEvent[]>([]);
  const { isAuthenticated, getAccessTokenSilently } = useAuth0();
  const baseUrl = import.meta.env.VITE_API_BASE_URL as string;

  useEffect(() => {
    loadTasks().then(setTasks);
    loadEvents().then(setEvents);
  }, []);

  useEffect(() => {
    saveTasks(tasks);
  }, [tasks]);

  useEffect(() => {
    saveEvents(events);
  }, [events]);

  useEffect(() => {
    if (!isAuthenticated) return;
    async function fetchRemote() {
      try {
        const token = await getAccessTokenSilently();
        const res = await fetch(`${baseUrl}/tasks`, {
          headers: { Authorization: `Bearer ${token}` }
        });
        if (res.ok) {
          const data: Task[] = await res.json();
          setTasks(data);
          saveTasks(data);
        }
      } catch (err) {
        console.error(err);
      }
    }
    fetchRemote();
  }, [isAuthenticated, baseUrl, getAccessTokenSilently]);

  useEffect(() => {
    if (!isAuthenticated || events.length === 0) return;
    const id = setTimeout(async () => {
      try {
        const token = await getAccessTokenSilently();
        await fetch(`${baseUrl}/events`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            Authorization: `Bearer ${token}`
          },
          body: JSON.stringify(events)
        });
        setEvents([]);
        saveEvents([]);
      } catch (err) {
        console.error(err);
      }
    }, 500);
    return () => clearTimeout(id);
  }, [events, isAuthenticated, baseUrl, getAccessTokenSilently]);

  function addTask(partial: Omit<Task, 'id'>) {
    const id = uuid();
    const newTask: Task = { id, ...partial, done: false };
    setTasks((t) => [...t, newTask]);
    const ev: TaskEvent = {
      id: uuid(),
      taskId: id,
      type: 'task-created',
      data: partial,
      time: Date.now()
    };
    setEvents((e) => [...e, ev]);
  }

  function updateTask(id: string, changes: Partial<Task>) {
    setTasks((t) => t.map((task) => (task.id === id ? { ...task, ...changes } : task)));
    const ev: TaskEvent = {
      id: uuid(),
      taskId: id,
      type: 'task-updated',
      data: changes,
      time: Date.now()
    };
    setEvents((e) => [...e, ev]);
  }

  function completeTask(id: string) {
    setTasks((t) => t.map((task) => (task.id === id ? { ...task, done: true } : task)));
    const ev: TaskEvent = {
      id: uuid(),
      taskId: id,
      type: 'task-completed',
      time: Date.now()
    };
    setEvents((e) => [...e, ev]);
  }

  return { tasks, addTask, updateTask, completeTask };
}
