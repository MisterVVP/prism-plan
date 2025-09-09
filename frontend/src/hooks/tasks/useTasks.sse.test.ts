import { renderHook, act, waitFor } from '@testing-library/react';
import { vi, describe, it, expect } from 'vitest';
import { useTasks } from './useTasks';

vi.mock('@auth0/auth0-react', () => ({
  useAuth0: () => ({
    isAuthenticated: true,
    getAccessTokenSilently: vi.fn().mockResolvedValue('token'),
    loginWithRedirect: vi.fn(),
    user: { sub: 'user1' },
  }),
}));

class MockEventSource {
  static instance: MockEventSource | null = null;
  onmessage: ((ev: MessageEvent) => void) | null = null;
  constructor(url: string) {
    MockEventSource.instance = this;
  }
  emit(data: string) {
    this.onmessage?.({ data } as MessageEvent);
  }
  close() {}
}
(globalThis as any).EventSource = MockEventSource as any;

(globalThis as any).fetch = vi.fn().mockResolvedValue({
  ok: true,
  json: () => Promise.resolve([]),
}) as any;

describe('useTasks SSE', () => {
  it('ignores keep-alive messages', async () => {
    const errSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
    const { result, unmount } = renderHook(() => useTasks());

    await waitFor(() => expect(MockEventSource.instance).not.toBeNull());

    act(() => {
      MockEventSource.instance!.emit(': keep-alive');
    });

    expect(result.current.tasks).toHaveLength(0);

    act(() => {
      MockEventSource.instance!.emit('{"entityType":"task","data":[{"id":"1","title":"a","notes":"","category":"normal","order":0,"done":false}]}');
    });

    expect(result.current.tasks).toHaveLength(1);
    errSpy.mockRestore();
    unmount();
  });
});

