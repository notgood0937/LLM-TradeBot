<template>
  <div class="glass-panel p-4 h-full flex flex-col">
    <h2 class="text-sm font-bold text-gray-400 mb-4 uppercase tracking-wider border-b border-white/10 pb-2">Treasury Curve</h2>
    <div class="flex-1 min-h-[250px] relative">
      <canvas ref="canvasEl"></canvas>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import Chart from 'chart.js/auto'
import { useTradingStore } from '../../stores/tradingStore'

const canvasEl = ref<HTMLCanvasElement | null>(null)
const store = useTradingStore()
let chart: Chart | null = null

onMounted(() => {
  if (canvasEl.value) {
    chart = new Chart(canvasEl.value, {
      type: 'line',
      data: {
        labels: [],
        datasets: [{
          label: 'Equity (USDT)',
          data: [],
          borderColor: '#F0B90B',
          backgroundColor: 'rgba(240, 185, 11, 0.1)',
          fill: true,
          tension: 0.3,
          borderWidth: 2,
          pointRadius: 0
        }]
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: { legend: { display: false } },
        scales: {
          x: { display: true, grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: '#94a3b8' } },
          y: { display: true, grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: '#94a3b8' } }
        },
        interaction: { mode: 'index', intersect: false }
      }
    })
  }
})

// Quick reactive charting
watch(() => store.account.realtime_balance, (newVal) => {
  if (!chart) return
  const numVal = parseFloat(String(newVal).replace(/[^0-9.-]+/g,""))
  if (isNaN(numVal)) return
  
  const time = new Date().toLocaleTimeString('en-US', { hour12: false })
  chart.data.labels?.push(time)
  chart.data.datasets[0].data.push(numVal)
  
  if (chart.data.labels && chart.data.labels.length > 50) {
    chart.data.labels.shift()
    chart.data.datasets[0].data.shift()
  }
  chart.update('none')
})
</script>
