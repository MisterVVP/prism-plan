import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import TaskModal from '.';

describe('TaskModal', () => {
  it('renders when open', () => {
    render(<TaskModal isOpen onClose={() => {}} addTask={() => {}} />);
    expect(screen.getByText('Add Task')).toBeTruthy();
  });
});

