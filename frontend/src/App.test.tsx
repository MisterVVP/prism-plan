import { render, screen, waitFor } from '@testing-library/react';
import type { Mock } from 'vitest';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import App from './App';
import { aria } from './aria';
import { useAuth0 } from '@auth0/auth0-react';

vi.mock('./hooks', () => ({
  useTasks: () => ({
    tasks: [],
    addTask: vi.fn(),
    updateTask: vi.fn(),
    completeTask: vi.fn(),
    reopenTask: vi.fn(),
  }),
  useLoginUser: () => {},
  useSettings: () => ({
    settings: { tasksPerCategory: 3, showDoneTasks: true },
    updateSettings: vi.fn(),
  }),
}));

vi.mock('@auth0/auth0-react', () => ({
  useAuth0: vi.fn(),
}));

const loginWithRedirect = vi.fn();
const logout = vi.fn();
const getAccessTokenSilently = vi.fn();
const mockedUseAuth0 = useAuth0 as unknown as Mock;

describe('App', () => {
  beforeEach(() => {
    loginWithRedirect.mockReset();
    logout.mockReset();
    getAccessTokenSilently.mockReset();
    mockedUseAuth0.mockReset();
  });

  it('redirects unauthenticated users to the login page', async () => {
    mockedUseAuth0.mockReturnValue({
      loginWithRedirect,
      logout,
      isAuthenticated: false,
      isLoading: false,
      user: null,
      getAccessTokenSilently,
      error: undefined,
    });

    render(<App />);

    await waitFor(() => {
      expect(loginWithRedirect).toHaveBeenCalledTimes(1);
    });

    expect(screen.getByText(/redirecting to sign in/i)).toBeTruthy();
    expect(
      screen.queryByRole('banner', { name: aria.header['aria-label'] })
    ).toBeNull();
  });

  it('renders the application layout once authenticated', () => {
    mockedUseAuth0.mockReturnValue({
      loginWithRedirect,
      logout,
      isAuthenticated: true,
      isLoading: false,
      user: { picture: 'avatar.png' },
      getAccessTokenSilently,
      error: undefined,
    });

    render(<App />);

    expect(loginWithRedirect).not.toHaveBeenCalled();
    expect(
      screen.getByRole('banner', { name: aria.header['aria-label'] })
    ).toBeTruthy();
    expect(
      screen.getByRole('main', { name: aria.main['aria-label'] })
    ).toBeTruthy();
    expect(
      screen.getByRole('contentinfo', { name: aria.footer['aria-label'] })
    ).toBeTruthy();
  });
});
