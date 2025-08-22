import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import UserMenu, { aria } from '.';

describe('UserMenu', () => {
  it('calls onLogin when login icon clicked', () => {
    const onLogin = vi.fn();
    render(<UserMenu isAuthenticated={false} onLogin={onLogin} onLogout={() => {}} />);
    const btn = screen.getByRole('button', { name: aria.loginButton['aria-label'] });
    fireEvent.click(btn);
    expect(onLogin).toHaveBeenCalled();
    expect(btn.getAttribute('aria-label')).toBe(aria.loginButton['aria-label']);
  });

  it('renders avatar when authenticated', () => {
    render(<UserMenu isAuthenticated userPicture="pic" onLogin={() => {}} onLogout={() => {}} />);
    expect(screen.getByAltText(aria.avatar.alt)).toBeTruthy();
  });
});
