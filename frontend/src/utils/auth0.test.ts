import { describe, it, expect, afterEach, vi } from 'vitest';
import { ungzip } from 'pako';
import { fetchWithAccessTokenRetry } from './auth0';

describe('fetchWithAccessTokenRetry', () => {
  const originalFetch = global.fetch;

  afterEach(() => {
    if (originalFetch) {
      global.fetch = originalFetch;
    } else {
      // @ts-expect-error allow deleting mocked fetch
      delete global.fetch;
    }
    vi.restoreAllMocks();
  });

  it('compresses JSON request bodies and sets gzip headers', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      status: 200,
      ok: true,
      json: () => Promise.resolve({}),
    });
    global.fetch = fetchMock as unknown as typeof fetch;

    const getAccessTokenSilently = vi.fn().mockResolvedValue('token');
    const payload = JSON.stringify([{ action: 'test' }]);

    await fetchWithAccessTokenRetry(
      getAccessTokenSilently,
      'aud',
      'https://example.test/commands',
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: payload,
      }
    );

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const call = fetchMock.mock.calls[0];
    const init = call?.[1] as RequestInit | undefined;
    expect(init?.body).toBeInstanceOf(Uint8Array);
    const compressed = init?.body as Uint8Array;
    const restored = new TextDecoder().decode(ungzip(compressed));
    expect(restored).toBe(payload);
    const headers = init?.headers as Headers;
    expect(headers.get('Content-Encoding')).toBe('gzip');
    expect(headers.get('Accept')).toBe('application/json');
    expect(headers.get('Authorization')).toBe('Bearer token');
  });

  it('preserves non-JSON bodies without compression', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      status: 200,
      ok: true,
      text: () => Promise.resolve('ok'),
    });
    global.fetch = fetchMock as unknown as typeof fetch;

    const getAccessTokenSilently = vi.fn().mockResolvedValue('token');
    const body = 'plain text';

    await fetchWithAccessTokenRetry(
      getAccessTokenSilently,
      'aud',
      'https://example.test/plain',
      {
        method: 'POST',
        headers: { 'Content-Type': 'text/plain' },
        body,
      }
    );

    const call = fetchMock.mock.calls[0];
    const init = call?.[1] as RequestInit | undefined;
    expect(init?.body).toBe(body);
    const headers = init?.headers as Headers;
    expect(headers.get('Content-Encoding')).toBeNull();
  });

  it('adds Accept header for JSON reads', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      status: 200,
      ok: true,
      json: () => Promise.resolve({}),
    });
    global.fetch = fetchMock as unknown as typeof fetch;

    const getAccessTokenSilently = vi.fn().mockResolvedValue('token');

    await fetchWithAccessTokenRetry(
      getAccessTokenSilently,
      'aud',
      'https://example.test/tasks'
    );

    const call = fetchMock.mock.calls[0];
    const init = call?.[1] as RequestInit | undefined;
    const headers = init?.headers as Headers;
    expect(headers.get('Accept')).toBe('application/json');
  });
});
