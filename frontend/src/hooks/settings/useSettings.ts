import { useEffect, useReducer } from "react";
import { useAuth0 } from "@auth0/auth0-react";
import type { Settings } from "../../types";
import { settingsReducer, settingsInitialState } from "../../reducers";
import { subscribe } from "../../stream";
import {
  fetchWithAccessTokenRetry,
  getStableAccessToken,
} from "../../utils/auth0";

export function useSettings() {
  const [state, dispatch] = useReducer(settingsReducer, settingsInitialState);
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
        const { response } = await fetchWithAccessTokenRetry(
          getAccessTokenSilently,
          audience,
          `${apiBaseUrl}/settings`
        );
        if (response.ok) {
          const data = await response.json();
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
    return subscribe(
      () =>
        getStableAccessToken(getAccessTokenSilently, audience).then(
          ({ token }) => token
        ),
      streamUrl,
      (msg) => {
        if (msg.entityType === "user-settings") {
          dispatch({ type: "merge-settings", settings: msg.data as Partial<Settings> });
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

  function updateSettings(changes: Partial<Settings>) {
    if (!user?.sub) return;
    dispatch({ type: "update-settings", userId: user.sub, settings: changes });
  }

  return { settings, updateSettings };
}
