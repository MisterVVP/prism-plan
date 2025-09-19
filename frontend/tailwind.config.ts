import type { Config } from 'tailwindcss';
import { palette } from './src/modules/palette';

export default <Config>{
  content: [
    './index.html',
    './src/**/*.{ts,tsx}'
  ],
  safelist: [
    { pattern: /(bg|text)-(critical|fun|important|normal)/ }
  ],
  theme: {
    extend: {
      colors: palette
    }
  },
  plugins: []
};
