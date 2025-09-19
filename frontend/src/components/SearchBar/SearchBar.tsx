import { memo } from 'react';
import { aria } from '.';

interface SearchBarProps {
  value: string;
  onChange: (value: string) => void;
}

function SearchBar({ value, onChange }: SearchBarProps) {
  return (
    <div className="flex-1 px-1 sm:px-2 lg:px-4">
      <input
        type="text"
        placeholder="Search..."
        value={value}
        onChange={(e) => onChange(e.target.value)}
        {...aria.input}
        className="w-full rounded-md border border-gray-300 px-1 py-1 text-xs focus:border-indigo-500 focus:ring-indigo-500 sm:px-2 sm:py-1 sm:text-sm"
      />
    </div>
  );
}

export default memo(SearchBar);
