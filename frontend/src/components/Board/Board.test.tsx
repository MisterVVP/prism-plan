import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import Board from '.';

describe('Board', () => {
  it('renders categories', () => {
    render(<Board tasks={[]} updateTask={() => {}} completeTask={() => {}} />);
    expect(screen.getByText('Critical')).toBeTruthy();
  });
});

