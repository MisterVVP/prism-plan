import { useEffect, useReducer } from "react";
import { useAuth0 } from "@auth0/auth0-react";
import type { Task } from "../../types";
import { tasksReducer, initialState } from "../../reducers";
import { subscribe } from "../../stream";

export function useTasks() {
  const [state, dispatch] = useReducer(tasksReducer, initialState);
  const { tasks, commands } = state;
  const { isAuthenticated, getAccessTokenSilently, loginWithRedirect } =
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
          const raw = await res.json();
          const data: Task[] = Array.isArray(raw) ? raw : [];
          dispatch({ type: "set-tasks", tasks: data });
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
    return subscribe(
      () =>
        getAccessTokenSilently({
          authorizationParams: {
            audience,
            scope: "openid profile email offline_access",
          },
        }),
      streamUrl,
      (msg) => {
        if (msg.entityType === "task" && Array.isArray(msg.data)) {
          dispatch({ type: "merge-tasks", tasks: msg.data as Task[] });
        }
      }
    );
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
        const res = await fetch(`${apiBaseUrl}/commands`, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${token}`,
          },
          body: JSON.stringify(commands),
        });
        const { idempotencyKeys } = await res.json();
        if (!cancelled) {
          dispatch({ type: "set-idempotency-keys", keys: idempotencyKeys });
          if (res.ok) {
            dispatch({ type: "clear-commands" });
          }
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

  function addTask(partial: Omit<Task, "id" | "order" | "done">) {
    dispatch({ type: "add-task", partial });
  }

  function updateTask(id: string, changes: Partial<Task>) {
    dispatch({ type: "update-task", id, changes });
  }

  function completeTask(id: string) {
    dispatch({ type: "complete-task", id });
  }

  function reopenTask(id: string) {
    dispatch({ type: "reopen-task", id });
  }

  return { tasks, addTask, updateTask, completeTask, reopenTask };
}
