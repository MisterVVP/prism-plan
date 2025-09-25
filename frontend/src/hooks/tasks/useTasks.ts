import { useEffect, useReducer } from "react";
import { useAuth0 } from "@auth0/auth0-react";
import type { Task } from '@modules/types';
import { tasksReducer, initialState } from '@reducers';
import { subscribe } from '@modules/stream';
import {
  fetchWithAccessTokenRetry,
  getStableAccessToken,
} from '@utils';

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
        const aggregated: Task[] = [];
        let pageToken: string | undefined;
        const seenTokens = new Set<string>();
        while (true) {
          const url = new URL(`${apiBaseUrl}/tasks`);
          if (pageToken) {
            url.searchParams.set("pageToken", pageToken);
          }
          const { response } = await fetchWithAccessTokenRetry(
            getAccessTokenSilently,
            audience,
            url.toString()
          );
          if (!response.ok) {
            break;
          }
          const raw = await response.json();
          let pageTasks: Task[] = [];
          let nextToken: string | undefined;
          if (Array.isArray(raw)) {
            pageTasks = raw as Task[];
          } else if (raw && typeof raw === "object") {
            const maybe = raw as { tasks?: unknown; nextPageToken?: unknown };
            if (Array.isArray(maybe.tasks)) {
              pageTasks = maybe.tasks as Task[];
            }
            if (typeof maybe.nextPageToken === "string") {
              const trimmed = maybe.nextPageToken.trim();
              if (trimmed) {
                nextToken = trimmed;
              }
            }
          } else {
            break;
          }
          aggregated.push(...pageTasks);
          if (!nextToken) {
            pageToken = undefined;
            break;
          }
          if (seenTokens.has(nextToken)) {
            console.warn(
              "Detected repeated pagination token while fetching tasks."
            );
            break;
          }
          seenTokens.add(nextToken);
          pageToken = nextToken;
        }
        dispatch({ type: "set-tasks", tasks: aggregated });
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
        getStableAccessToken(getAccessTokenSilently, audience).then(
          ({ token }) => token
        ),
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
        const { response } = await fetchWithAccessTokenRetry(
          getAccessTokenSilently,
          audience,
          `${apiBaseUrl}/commands`,
          {
            method: "POST",
            headers: {
              "Content-Type": "application/json",
            },
            body: JSON.stringify(commands),
          }
        );
        const { idempotencyKeys } = await response.json();
        if (!cancelled) {
          dispatch({ type: "set-idempotency-keys", keys: idempotencyKeys });
          if (response.ok) {
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
