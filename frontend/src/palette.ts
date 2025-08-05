export const palette = {
  critical: '#FF5252',
  fun: '#4CAF50',
  important: '#3F7FBF',
  normal: '#D2D2D2',
  done: '#9CA3AF',
  background: '#FFFFFF',
  foreground: '#111827'
} as const;

type Palette = typeof palette;
export type PaletteKey = keyof Palette;
