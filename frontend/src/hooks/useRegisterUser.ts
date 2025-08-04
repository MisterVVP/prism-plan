import { useEffect } from 'react';
import { useAuth0 } from '@auth0/auth0-react';
import { v4 as uuid } from 'uuid';

export function useRegisterUser() {
  const { isAuthenticated, user, getAccessTokenSilently } = useAuth0();
  const baseUrl = import.meta.env.VITE_API_BASE_URL as string;
  const audience = import.meta.env.VITE_AUTH0_AUDIENCE as string;

  useEffect(() => {
    if (!isAuthenticated || !user?.sub) return;
    async function register() {
      try {
        const token = await getAccessTokenSilently({
          authorizationParams: {
            audience,
            scope: 'openid profile email offline_access'
          }
        });
        const event = {
          id: uuid(),
          entityId: user?.sub,
          entityType: 'users',
          type: 'user-registered',
          data: { name: user?.name, email: user?.email },
          time: Date.now()
        };
        await fetch(`${baseUrl}/events`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            Authorization: `Bearer ${token}`
          },
          body: JSON.stringify([event])
        });
      } catch (err) {
        console.error(err);
      }
    }
    register();
  }, [isAuthenticated, user, baseUrl, getAccessTokenSilently, audience]);
}
