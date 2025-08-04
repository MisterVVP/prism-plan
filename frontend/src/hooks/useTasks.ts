import { useEffect, useState } from 'react';
import { useAuth0 } from '@auth0/auth0-react';
import { v4 as uuid } from 'uuid';
import type { Task, TaskEvent } from '../types';

export function useTasks() {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [events, setEvents] = useState<TaskEvent[]>([]);
  const { isAuthenticated, getAccessTokenSilently, loginWithRedirect, user } = useAuth0();
  const baseUrl = import.meta.env.VITE_API_BASE_URL as string;
  const audience = import.meta.env.VITE_AUTH0_AUDIENCE as string;
  const userId = user?.sub ?? null;

  useEffect(() => {
    if (!isAuthenticated) return;
    async function fetchRemote() {
      try {
        const token = await getAccessTokenSilently({
          authorizationParams: {
            audience,
            scope: 'openid profile email offline_access'
          }
        });
        const res = await fetch(`${baseUrl}/tasks`, {
          headers: { Authorization: `Bearer ${token}` }
        });
        if (res.ok) {
          const data: Task[] = await res.json();
          setTasks(data);
        }
      } catch (err) {
        if (err instanceof Error && err.message.includes('Missing Refresh Token')) {
          loginWithRedirect();
        } else {
          console.error(err);
        }
      }
    }
    fetchRemote();
    const interval = setInterval(fetchRemote, 60000);
    return () => clearInterval(interval);
  }, [isAuthenticated, baseUrl, getAccessTokenSilently, loginWithRedirect, audience]);

  useEffect(() => {
    if (!isAuthenticated || events.length === 0) return;
    let cancelled = false;
    async function flushEvents() {
      try {
        const token = await getAccessTokenSilently({
          authorizationParams: {
            audience,
            scope: 'openid profile email offline_access'
          }
        });
        await fetch(`${baseUrl}/events`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            Authorization: `Bearer ${token}`
          },
          body: JSON.stringify(events)
        });
        if (!cancelled) {
          setEvents([]);
        }
      } catch (err) {
        if (err instanceof Error && err.message.includes('Missing Refresh Token')) {
          loginWithRedirect();
        } else {
          console.error(err);
        }
      }
    }
    flushEvents();
    const interval = setInterval(flushEvents, 5000);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [events, isAuthenticated, baseUrl, getAccessTokenSilently, loginWithRedirect, audience]);

  function addTask(partial: Omit<Task, 'id'>) {
    const id = uuid();
    const newTask: Task = { id, ...partial, done: false };
    setTasks((t) => [...t, newTask]);
    const ev: TaskEvent = {
      id: uuid(),
      entityId: id,
      entityType: 'task',
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
      entityId: id,
      entityType: 'task',
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
      entityId: id,
      entityType: 'task',
      type: 'task-completed',
      time: Date.now()
    };
    setEvents((e) => [...e, ev]);
  }

  return { tasks, addTask, updateTask, completeTask };
}
