/** @type {import('tailwindcss').Config} */
// Tokens do mockup — os valores claro/escuro ficam em CSS variables (src/style.css),
// então as classes bg-surface, text-ink etc. já respondem ao toggle de tema.
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
        lime: '#C8F14B',
        pitch: { 1: '#082A1D', 2: '#0A3B28', 3: '#0F5238', 4: '#116040' },
      },
      fontFamily: {
        sans: ['Barlow', 'system-ui', 'sans-serif'],
        condensed: ['"Barlow Condensed"', 'sans-serif'],
      },
      boxShadow: {
        card: 'var(--shadow)',
      },
    },
  },
  plugins: [],
}
