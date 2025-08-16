import { useEffect, useState } from "react";
import { useAuth0 } from "@auth0/auth0-react";
import { v4 as uuid } from "uuid";
import type { Task, Command } from "../types";

export function useTasks() {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [commands, setCommands] = useState<Command[]>([]);
  const { isAuthenticated, getAccessTokenSilently, loginWithRedirect, user } =
    useAuth0();
  const apiBaseUrl =
    (import.meta.env.VITE_API_BASE_URL as string | undefined) ||
    `${window.location.origin}/api`;
  const streamUrl =
    (import.meta.env.VITE_STREAM_URL as string | undefined) ||
    `${window.location.origin}/stream`;
  const audience = import.meta.env.VITE_AUTH0_AUDIENCE as string;

  useEffect(() => {
    if (!isAuthenticated) return;
    async function fetchRemote() {
      try {
        const token = await getAccessTokenSilently({
          authorizationParams: {
            audience,
            scope: "openid profile email offline_access",
          },
        });
        const res = await fetch(`${apiBaseUrl}/tasks`, {
          headers: { Authorization: `Bearer ${token}` },
        });
        if (res.ok) {
          const data: Task[] = await res.json();
          setTasks(data);
        }
      } catch (err) {
        if (
          err instanceof Error &&
          err.message.includes("Missing Refresh Token")
        ) {
          loginWithRedirect();
        } else {
          console.error(err);
        }
      }
    }
    fetchRemote();
    const interval = setInterval(fetchRemote, 60000);
    return () => clearInterval(interval);
  }, [
    isAuthenticated,
    apiBaseUrl,
    getAccessTokenSilently,
    loginWithRedirect,
    audience,
  ]);

  useEffect(() => {
    if (!isAuthenticated) return;
    let source: EventSource | null = null;
    async function connect() {
      try {
        const token = await getAccessTokenSilently({
          authorizationParams: {
            audience,
            scope: "openid profile email offline_access",
          },
        });
        source = new EventSource(`${streamUrl}?token=${token}`);
        source.onmessage = (ev) => {
          const data: Task[] = JSON.parse(ev.data);
          setTasks(data);
        };
      } catch (err) {
        console.error(err);
      }
    }
    connect();
    return () => {
      if (source) source.close();
    };
  }, [isAuthenticated, streamUrl, getAccessTokenSilently, audience]);

  useEffect(() => {
    if (!isAuthenticated || commands.length === 0) return;
    let cancelled = false;
    async function flushCommands() {
      try {
        const token = await getAccessTokenSilently({
          authorizationParams: {
            audience,
            scope: "openid profile email offline_access",
          },
        });
        await fetch(`${apiBaseUrl}/commands`, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${token}`,
          },
          body: JSON.stringify(commands),
        });
        if (!cancelled) {
          setCommands([]);
        }
      } catch (err) {
        if (
          err instanceof Error &&
          err.message.includes("Missing Refresh Token")
        ) {
          loginWithRedirect();
        } else {
          console.error(err);
        }
      }
    }
    flushCommands();
    const interval = setInterval(flushCommands, 5000);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [
    commands,
    isAuthenticated,
    apiBaseUrl,
    getAccessTokenSilently,
    loginWithRedirect,
    audience,
  ]);

  function addTask(partial: Omit<Task, "id">) {
    const id = uuid();
    const existingOrders = [
      ...tasks
        .filter((t) => t.category === partial.category)
        .map((t) => t.order ?? -1),
      ...commands
        .filter(
          (c) =>
            c.type === "create-task" &&
            (c.data as any).category === partial.category
        )
        .map((c) => ((c.data as any).order as number) ?? -1),
    ];
    const nextOrder = (existingOrders.length ? Math.max(...existingOrders) : -1) + 1;
    const newTask: Task = { id, ...partial, order: nextOrder, done: false };
    setTasks((t) => [...t, newTask]);
    const cmd: Command = {
      id: uuid(),
      entityId: id,
      entityType: "task",
      type: "create-task",
      data: { ...partial, order: nextOrder },
    };
    setCommands((e) => [...e, cmd]);
  }

  function updateTask(id: string, changes: Partial<Task>) {
    setTasks((t) =>
      t.map((task) => (task.id === id ? { ...task, ...changes } : task)),
    );
    const cmd: Command = {
      id: uuid(),
      entityId: id,
      entityType: "task",
      type: "update-task",
      data: changes,
    };
    setCommands((e) => [...e, cmd]);
  }

  function completeTask(id: string) {
    setTasks((t) =>
      t.map((task) => (task.id === id ? { ...task, done: true } : task)),
    );
    const cmd: Command = {
      id: uuid(),
      entityId: id,
      entityType: "task",
      type: "complete-task",
    };
    setCommands((e) => [...e, cmd]);
  }

  return { tasks, addTask, updateTask, completeTask };
}
