import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import TaskCard from '.';
import type { Task } from '../../types';

vi.mock('@dnd-kit/sortable', () => ({
  useSortable: () => ({
    attributes: {},
    listeners: {},
    setNodeRef: () => {},
    transform: null,
    transition: null
  })
}));

describe('TaskCard', () => {
  it('renders task title', () => {
    const task: Task = { id: '1', title: 'Sample', category: 'normal', notes: '', order: 0, done: false };
    render(<TaskCard task={task} />);
    expect(screen.getByText('Sample')).toBeTruthy();
  });

  it('fires onClick on a single click', () => {
    vi.useFakeTimers();
    const task: Task = { id: '1', title: 'Sample', category: 'normal', notes: '', order: 0, done: false };
    const handleClick = vi.fn();
    const handleDouble = vi.fn();
    const { container } = render(<TaskCard task={task} onClick={handleClick} onDoubleClick={handleDouble} />);
    const card = container.firstElementChild as HTMLElement;
    fireEvent.click(card, { detail: 1 });
    expect(handleClick).not.toHaveBeenCalled();
    expect(handleDouble).not.toHaveBeenCalled();
    vi.advanceTimersByTime(251);
    expect(handleClick).toHaveBeenCalledTimes(1);
    expect(handleDouble).not.toHaveBeenCalled();
    vi.useRealTimers();
  });

  it('fires onDoubleClick on rapid double click', () => {
    vi.useFakeTimers();
    const task: Task = { id: '1', title: 'Sample', category: 'normal', notes: '', order: 0, done: false };
    const handleClick = vi.fn();
    const handleDouble = vi.fn();
    const { container } = render(<TaskCard task={task} onClick={handleClick} onDoubleClick={handleDouble} />);
    const card = container.firstElementChild as HTMLElement;
    fireEvent.click(card);
    fireEvent.click(card, { detail: 2 });
    vi.runAllTimers();
    expect(handleDouble).toHaveBeenCalledTimes(1);
    expect(handleClick).not.toHaveBeenCalled();
    vi.useRealTimers();
  });
});

