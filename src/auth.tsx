import { Auth0Provider } from '@auth0/auth0-react';
import React from 'react';

interface Props {
  children: React.ReactNode;
}

export function AuthProvider({ children }: Props) {
  const domain = import.meta.env.VITE_AUTH0_DOMAIN as string;
  const clientId = import.meta.env.VITE_AUTH0_CLIENT_ID as string;

  return (
    <Auth0Provider
      domain={domain}
      clientId={clientId}
      authorizationParams={{ redirect_uri: window.location.origin }}
    >
      {children}
    </Auth0Provider>
  );
}
