import { useEffect, useRef } from "react";
import { useAuth0 } from "@auth0/auth0-react";

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
        const tokenResponse = (await getAccessTokenSilently({
          authorizationParams: {
            audience,
            scope: "openid profile email offline_access",
          },
          detailedResponse: true,
        })) as any;
        const token: string = tokenResponse.access_token || tokenResponse;
        const expiresIn: number = tokenResponse.expires_in || 0;
        const command = {
          entityType: "user",
          type: "login-user",
          data: { name: user.name, email: user.email },
        };
        const res = await fetch(`${baseUrl}/commands`, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${token}`,
          },
          body: JSON.stringify([command]),
        });
        await res.json();
        const expiresAt = Date.now() + expiresIn * 1000;
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
