/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        primary: '#1E3A8A',
        secondary: '#06B6D4',
        accent: '#22C55E',
        warn: '#F59E0B',
        danger: '#EF4444',
        bg: '#F8FAFC',
      },
      fontFamily: {
        dm: ['"DM Sans"', 'ui-sans-serif', 'system-ui', 'sans-serif'],
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
