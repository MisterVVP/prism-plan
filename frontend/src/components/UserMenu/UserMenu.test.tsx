import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import UserMenu from '.';

describe('UserMenu', () => {
  it('calls onLogin when login icon clicked', () => {
    const onLogin = vi.fn();
    render(<UserMenu isAuthenticated={false} onLogin={onLogin} onLogout={() => {}} />);
    fireEvent.click(screen.getByRole('button', { name: /log in/i }));
    expect(onLogin).toHaveBeenCalled();
  });

  it('renders avatar when authenticated', () => {
    render(<UserMenu isAuthenticated userPicture="pic" onLogin={() => {}} onLogout={() => {}} />);
    expect(screen.getByAltText('User avatar')).toBeTruthy();
  });
});
