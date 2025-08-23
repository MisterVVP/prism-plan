import { useEffect, useReducer } from "react";
import { useAuth0 } from "@auth0/auth0-react";
import { v4 as uuid } from "uuid";
import type { Settings } from "../../types";
import { settingsReducer, initialState } from "../../reducers";

export function useSettings() {
  const [state, dispatch] = useReducer(settingsReducer, initialState);
  const { settings, commands } = state;
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
        const res = await fetch(`${apiBaseUrl}/settings`, {
          headers: { Authorization: `Bearer ${token}` },
        });
        if (res.ok) {
          const data = await res.json();
          dispatch({ type: "set-settings", settings: data as Settings });
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
          try {
            const msg = JSON.parse(ev.data);
            if (msg.entityType === "user-settings") {
              dispatch({ type: "merge-settings", settings: msg.data as Partial<Settings> });
            }
          } catch (e) {
            console.error(e);
          }
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
          dispatch({ type: "clear-commands" });
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

  function updateSettings(changes: Partial<Settings>) {
    if (!user?.sub) return;
    dispatch({
      type: "update-settings",
      commandId: uuid(),
      userId: user.sub,
      settings: changes,
    });
  }

  return { settings, updateSettings };
}
