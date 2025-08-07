import { useEffect, useRef } from "react";
import { useAuth0 } from "@auth0/auth0-react";
import { v4 as uuid } from "uuid";

export function useLoginUser() {
  const { isAuthenticated, user, getAccessTokenSilently } = useAuth0();
  const baseUrl =
    (import.meta.env.VITE_API_BASE_URL as string | undefined) ||
    `${window.location.origin}/api`;
  const audience = import.meta.env.VITE_AUTH0_AUDIENCE as string;
  const lastUserId = useRef<string | null>(null);

  useEffect(() => {
    if (!isAuthenticated || !user?.sub) return;
    if (lastUserId.current === user.sub) return;
    async function login() {
      try {
        const token = await getAccessTokenSilently({
          authorizationParams: {
            audience,
            scope: "openid profile email offline_access",
          },
        });
        const command = {
          id: uuid(),
          entityId: user.sub,
          entityType: "user",
          type: "login-user",
          data: { name: user.name, email: user.email },
        };
        await fetch(`${baseUrl}/commands`, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${token}`,
          },
          body: JSON.stringify([command]),
        });
      } catch (err) {
        console.error(err);
      }
    }
    login();
    lastUserId.current = user.sub;
  }, [isAuthenticated, user, baseUrl, getAccessTokenSilently, audience]);
}
