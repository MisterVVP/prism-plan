import { describe, it, expect, vi } from 'vitest';
import { subscribe } from './stream';

vi.mock('@auth0/auth0-react', () => ({}));

describe('stream subscribe', () => {
  it('reuses single EventSource connection', async () => {
    let instances = 0;
    class MockES {
      onmessage: ((ev: MessageEvent) => void) | null = null;
      constructor(url: string) {
        instances++;
      }
      close() {}
    }
    (globalThis as any).EventSource = MockES as any;
    const tokenProvider = () => Promise.resolve('token');
    const unsubscribe1 = subscribe(tokenProvider, '/s', () => {});
    const unsubscribe2 = subscribe(tokenProvider, '/s', () => {});
    await Promise.resolve();
    expect(instances).toBe(1);
    unsubscribe1();
    unsubscribe2();
  });
});
