import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import App from './App';
import { aria } from './aria';

vi.mock('./hooks', () => ({
  useTasks: () => ({
    tasks: [],
    addTask: vi.fn(),
    updateTask: vi.fn(),
    completeTask: vi.fn()
  }),
  useLoginUser: () => {},
  useSettings: () => ({
    settings: { tasksPerCategory: 3, showDoneTasks: true },
    updateSettings: vi.fn()
  })
}));

vi.mock('@auth0/auth0-react', () => ({
  useAuth0: () => ({
    loginWithRedirect: vi.fn(),
    logout: vi.fn(),
    isAuthenticated: false,
    user: null,
    getAccessTokenSilently: vi.fn()
  })
}));

describe('App', () => {
  it('applies aria attributes to header, main and footer', () => {
    render(<App />);
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
