export default {
  content: [
    "./index.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        'nofx-gold': '#F0B90B',
        'nofx-gold-dim': 'rgba(240, 185, 11, 0.1)',
        'nofx-bg': '#05070A',
        'nofx-bg-lighter': '#0E1217',
        'panel-bg': 'rgba(14, 18, 23, 0.6)',
        'accent-green': '#0ECB81',
        'accent-red': '#F6465D',
      },
      fontFamily: {
        mono: ['IBM Plex Mono', 'monospace'],
        sans: ['Inter', 'sans-serif'],
      }
    },
  },
  plugins: [],
}
