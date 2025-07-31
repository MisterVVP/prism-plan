import type { Config } from 'tailwindcss';

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
      colors: {
        critical: '#FF5252',
        fun: '#4CAF50',
        important: '#3F7FBF',
        normal: '#D2D2D2'
      }
    }
  },
  plugins: []
};
