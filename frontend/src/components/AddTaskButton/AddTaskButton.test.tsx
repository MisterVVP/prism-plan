import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import AddTaskButton from '.';

describe('AddTaskButton', () => {
  it('calls onAdd when clicked', () => {
    const onAdd = vi.fn();
    render(<AddTaskButton onAdd={onAdd} />);
    fireEvent.click(screen.getByRole('button', { name: /add task/i }));
    expect(onAdd).toHaveBeenCalled();
  });
});
