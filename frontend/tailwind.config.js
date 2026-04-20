/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        primary: '#3A7BFF',
        secondary: '#06B6D4',
        accent: '#34C759',
        warn: '#F59E0B',
        danger: '#EF4444',
        surface: '#F5F7FA',
        bg: '#F5F7FA',
        dark: {
          bg: '#1a1a2e',
          card: '#16213e',
          surface: '#0f3460',
        },
      },
      fontFamily: {
        main: ['Nunito', 'ui-sans-serif', 'system-ui', 'sans-serif'],
        dm: ['Nunito', 'ui-sans-serif', 'system-ui', 'sans-serif'],
      },
      keyframes: {
        glow: { '0%,100%': { opacity: 0.6 }, '50%': { opacity: 1 } },
        fadeIn: { from: { opacity: 0, transform: 'translateY(8px)' }, to: { opacity: 1, transform: 'translateY(0)' } },
      },
      animation: {
        glow: 'glow 3s ease-in-out infinite',
        fadeIn: 'fadeIn 0.3s ease',
      },
    },
  },
  plugins: [],
};
