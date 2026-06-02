/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      fontFamily: {
        sans: ['"PingFang SC"', '"Microsoft YaHei"', '"Noto Sans SC"', 'system-ui', 'sans-serif'],
      },
      colors: {
        scholar: {
          bg: '#faf8f5',
          card: '#ffffff',
          border: '#e8e0d5',
          accent: '#8b4513',
          accentHover: '#6b3410',
          navy: '#1a365d',
          text: '#1a1a1a',
          muted: '#6b7280',
          divider: '#e5e0d8',
        },
      },
    },
  },
  plugins: [require('@tailwindcss/typography')],
};
