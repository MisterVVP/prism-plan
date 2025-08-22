import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import SearchBar, { aria } from '.';

describe('SearchBar', () => {
  it('calls onChange with new value', () => {
    const onChange = vi.fn();
    render(<SearchBar value="" onChange={onChange} />);
    const input = screen.getByRole('textbox', { name: aria.input['aria-label'] });
    fireEvent.change(input, { target: { value: 'abc' } });
    expect(onChange).toHaveBeenCalledWith('abc');
    expect(input.getAttribute('aria-label')).toBe(aria.input['aria-label']);
  });
});
