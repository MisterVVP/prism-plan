import { renderHook, act, waitFor } from '@testing-library/react';
import { vi, describe, it, expect } from 'vitest';
import { useSettings } from './useSettings';

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
  json: () => Promise.resolve({ tasksPerCategory: 3, showDoneTasks: false }),
}) as any;

describe('useSettings SSE', () => {
  it('ignores keep-alive messages', async () => {
    const errSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
    const { result, unmount } = renderHook(() => useSettings());

    await waitFor(() => expect(MockEventSource.instance).not.toBeNull());

    act(() => {
      MockEventSource.instance!.emit(': keep-alive');
    });
    expect(result.current.settings).toEqual({ tasksPerCategory: 3, showDoneTasks: false });

    act(() => {
      MockEventSource.instance!.emit('{"entityType":"user-settings","data":{"showDoneTasks":true}}');
    });
    expect(result.current.settings.showDoneTasks).toBe(true);

    errSpy.mockRestore();
    unmount();
  });
});

