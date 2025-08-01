import { useEffect } from 'react';
import { useAuth0 } from '@auth0/auth0-react';

export function useRegisterUser() {
  const { isAuthenticated, user, getAccessTokenSilently } = useAuth0();
  const baseUrl = import.meta.env.VITE_API_BASE_URL as string;

  useEffect(() => {
    if (!isAuthenticated) return;
    async function register() {
      try {
        const token = await getAccessTokenSilently();
        await fetch(`${baseUrl}/user`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            Authorization: `Bearer ${token}`
          },
          body: JSON.stringify({ name: user?.name, email: user?.email })
        });
      } catch (err) {
        console.error(err);
      }
    }
    register();
  }, [isAuthenticated, user, baseUrl, getAccessTokenSilently]);
}
