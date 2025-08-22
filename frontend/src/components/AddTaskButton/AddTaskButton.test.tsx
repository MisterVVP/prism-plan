import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import AddTaskButton, { aria } from '.';

describe('AddTaskButton', () => {
  it('calls onAdd when clicked', () => {
    const onAdd = vi.fn();
    render(<AddTaskButton onAdd={onAdd} />);
    const button = screen.getByRole('button', { name: aria.button['aria-label'] });
    fireEvent.click(button);
    expect(onAdd).toHaveBeenCalled();
    expect(button.getAttribute('aria-label')).toBe(aria.button['aria-label']);
  });
});
