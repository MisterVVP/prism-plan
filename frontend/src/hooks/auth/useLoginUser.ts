import { useEffect, useRef } from "react";
import { useAuth0 } from "@auth0/auth0-react";
import { fetchWithAccessTokenRetry } from '@utils';

export function useLoginUser() {
  const { isAuthenticated, user, getAccessTokenSilently } = useAuth0();
  const baseUrl =
    (import.meta.env.VITE_API_BASE_URL as string | undefined) ||
    `${window.location.origin}/api`;
  const audience = import.meta.env.VITE_AUTH0_AUDIENCE as string;
  const lastUserId = useRef<string | null>(null);
  const storageKey = "login-user";

  useEffect(() => {
    if (!isAuthenticated || !user?.sub) return;
    if (lastUserId.current === user.sub) return;

    const saved = localStorage.getItem(storageKey);
    if (saved) {
      try {
        const { userId, expiresAt } = JSON.parse(saved) as {
          userId: string;
          expiresAt: number;
        };
        if (userId === user.sub && expiresAt > Date.now()) {
          lastUserId.current = user.sub;
          return;
        }
      } catch {
        // ignore parse errors and treat as no saved login
      }
    }

    async function login() {
      try {
        const command = {
          entityType: "user",
          type: "login-user",
          data: { name: user.name, email: user.email },
        };
        const { response, token } = await fetchWithAccessTokenRetry(
          getAccessTokenSilently,
          audience,
          `${baseUrl}/commands`,
          {
            method: "POST",
            headers: {
              "Content-Type": "application/json",
            },
            body: JSON.stringify([command]),
          }
        );
        await response.json();
        const expiresAt =
          token.expiresAt ?? Date.now() + (token.expiresIn ?? 0) * 1000;
        localStorage.setItem(
          storageKey,
          JSON.stringify({ userId: user.sub, expiresAt })
        );
      } catch (err) {
        console.error(err);
      }
    }
    login();
    lastUserId.current = user.sub;
  }, [isAuthenticated, user, baseUrl, getAccessTokenSilently, audience]);
}
