import type {
  GetTokenSilentlyOptions,
  GetTokenSilentlyVerboseResponse,
} from "@auth0/auth0-react";

const DEFAULT_SCOPE = "openid profile email offline_access";
const TOKEN_ACTIVATION_TOLERANCE_MS = 1000;
const MAX_TOKEN_ACTIVATION_WAIT_MS = 10000;
const TOKEN_RETRY_ERRORS = [
  /token used before being issued/i,
  /token used before issued/i,
  /token is not active/i,
  /token not yet valid/i,
];

export interface TokenMetadata {
  token: string;
  expiresIn?: number;
  expiresAt?: number;
}

export interface AuthorizedFetchResult {
  response: Response;
  token: TokenMetadata;
}

type GetAccessTokenSilently = (
  options?: GetTokenSilentlyOptions
) => Promise<string | GetTokenSilentlyVerboseResponse>;

export async function getStableAccessToken(
  getAccessTokenSilently: GetAccessTokenSilently,
  audience: string,
  options?: GetTokenSilentlyOptions
): Promise<TokenMetadata> {
  const params: GetTokenSilentlyOptions = {
    detailedResponse: true,
    ...options,
    authorizationParams: {
      audience,
      scope: DEFAULT_SCOPE,
      ...options?.authorizationParams,
    },
  };

  const response = await getAccessTokenSilently(params);
  const token =
    typeof response === "string" ? response : response?.access_token ?? "";

  if (!token) {
    throw new Error("No access token returned from Auth0");
  }

  const payload = decodeJwtPayload(token);
  const issuedAt =
    typeof payload?.iat === "number" ? payload.iat * 1000 : undefined;
  const expiresAt =
    typeof payload?.exp === "number" ? payload.exp * 1000 : undefined;
  const expiresIn =
    typeof response === "string" ? undefined : response?.expires_in;

  if (issuedAt) {
    const now = Date.now();
    const waitMs = issuedAt - now;
    if (waitMs > 0) {
      const totalWait = Math.min(
        waitMs + TOKEN_ACTIVATION_TOLERANCE_MS,
        MAX_TOKEN_ACTIVATION_WAIT_MS
      );
      await delay(totalWait);
    }
  }

  return { token, expiresAt, expiresIn };
}

export async function fetchWithAccessTokenRetry(
  getAccessTokenSilently: GetAccessTokenSilently,
  audience: string,
  input: RequestInfo | URL,
  init: RequestInit = {},
  options?: {
    tokenOptions?: GetTokenSilentlyOptions;
    retryDelaysMs?: number[];
  }
): Promise<AuthorizedFetchResult> {
  const delays = options?.retryDelaysMs ?? [1000, 3000];
  let attempt = 0;

  while (attempt <= delays.length) {
    const token = await getStableAccessToken(
      getAccessTokenSilently,
      audience,
      options?.tokenOptions
    );

    const headers = new Headers(init.headers ?? {});
    if (!headers.has("Authorization")) {
      headers.set("Authorization", `Bearer ${token.token}`);
    }

    const response = await fetch(input, { ...init, headers });

    if (response.status !== 401) {
      return { response, token };
    }

    const shouldRetry = await isTokenActivationError(response);
    if (!shouldRetry || attempt >= delays.length) {
      return { response, token };
    }

    await delay(delays[attempt]);
    attempt += 1;
  }

  throw new Error("Failed to complete authorized request");
}

function decodeJwtPayload(token: string): Record<string, unknown> | null {
  const [, payload] = token.split(".");
  if (!payload) return null;
  try {
    const normalized = payload.replace(/-/g, "+").replace(/_/g, "/");
    const padded = normalized.padEnd(normalized.length + (4 - (normalized.length % 4)) % 4, "=");
    const json = atob(padded);
    return JSON.parse(json) as Record<string, unknown>;
  } catch {
    return null;
  }
}

async function isTokenActivationError(response: Response): Promise<boolean> {
  try {
    const clone = response.clone();
    const text = await clone.text();
    return TOKEN_RETRY_ERRORS.some((pattern) => pattern.test(text));
  } catch {
    return false;
  }
}

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

export { DEFAULT_SCOPE };