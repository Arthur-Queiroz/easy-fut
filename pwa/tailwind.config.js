/** @type {import('tailwindcss').Config} */
// Tokens do design system Easy Fut — os valores claro/escuro ficam em CSS variables.
export default {
  content: ['./index.html', './src/**/*.{vue,ts}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        canvas: 'var(--canvas)',
        bg: 'var(--bg)',
        surface: 'var(--surface)',
        surface2: 'var(--surface2)',
        border: 'var(--border)',
        ink: 'var(--ink)',
        ink2: 'var(--ink2)',
        ink3: 'var(--ink3)',
        brand: 'var(--brand)',
        brandInk: 'var(--brand-ink)',
        brandSoft: 'var(--brand-soft)',
        warn: 'var(--warn)',
        warnBg: 'var(--warn-bg)',
        danger: 'var(--danger)',
        dangerBg: 'var(--danger-bg)',
        info: 'var(--info)',
        infoBg: 'var(--info-bg)',
        lime: 'var(--lime)',
        pitch: { 1: '#0c100f', 2: '#13251f', 3: '#0f3325', 4: '#1a6644' },
      },
      fontFamily: {
        sans: ['Manrope', 'system-ui', 'sans-serif'],
        condensed: ['"Barlow Condensed"', 'sans-serif'],
      },
      boxShadow: {
        card: 'var(--shadow)',
      },
    },
  },
  plugins: [],
}
