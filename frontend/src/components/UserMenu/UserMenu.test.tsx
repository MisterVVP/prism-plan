import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { describe, it, expect, vi, afterEach } from 'vitest';
import UserMenu, { aria } from '.';

vi.mock('@headlessui/react', () => {
  const Menu = ({ children }: any) => <div>{children}</div>;
  Menu.Button = ({ children, ...props }: any) => (
    <button type="button" {...props}>
      {children}
    </button>
  );
  Menu.Items = ({ children }: any) => <div>{children}</div>;
  Menu.Item = ({ children }: any) => <div>{typeof children === 'function' ? children({ active: false }) : children}</div>;
  return {
    Menu,
    Transition: ({ children }: any) => <>{children}</>,
  };
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

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

  it('throttles tasks per category updates while typing', () => {
    vi.useFakeTimers();
    const onUpdateSettings = vi.fn();
    render(
      <UserMenu
        isAuthenticated
        userPicture="pic"
        onLogin={() => {}}
        onLogout={() => {}}
        settings={{ tasksPerCategory: 3, showDoneTasks: false }}
        onUpdateSettings={onUpdateSettings}
      />
    );
    const input = screen.getByLabelText('Tasks per category');
    fireEvent.change(input, { target: { value: '4' } });
    vi.advanceTimersByTime(300);
    expect(onUpdateSettings).not.toHaveBeenCalled();
    fireEvent.change(input, { target: { value: '5' } });
    vi.advanceTimersByTime(399);
    expect(onUpdateSettings).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);
    expect(onUpdateSettings).toHaveBeenCalledTimes(1);
    expect(onUpdateSettings).toHaveBeenCalledWith({ tasksPerCategory: 5 });
  });

  it('flushes pending update on blur', () => {
    vi.useFakeTimers();
    const onUpdateSettings = vi.fn();
    render(
      <UserMenu
        isAuthenticated
        userPicture="pic"
        onLogin={() => {}}
        onLogout={() => {}}
        settings={{ tasksPerCategory: 3, showDoneTasks: false }}
        onUpdateSettings={onUpdateSettings}
      />
    );
    const input = screen.getByLabelText('Tasks per category');
    fireEvent.change(input, { target: { value: '6' } });
    fireEvent.blur(input);
    expect(onUpdateSettings).toHaveBeenCalledTimes(1);
    expect(onUpdateSettings).toHaveBeenCalledWith({ tasksPerCategory: 6 });
  });
});
