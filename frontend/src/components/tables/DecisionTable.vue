<template>
  <div class="glass-panel p-4 h-full flex flex-col">
    <h2 class="text-sm font-bold text-gray-400 mb-4 uppercase tracking-wider border-b border-white/10 pb-2">Archive Table</h2>
    
    <div class="flex gap-2 mb-3">
      <select v-model="filterSymbol" class="bg-black/40 text-xs border border-white/10 rounded p-1">
        <option value="all">All Symbols</option>
        <option v-for="sym in symbols" :key="sym" :value="sym">{{ sym }}</option>
      </select>
      <select v-model="filterAction" class="bg-black/40 text-xs border border-white/10 rounded p-1">
        <option value="all">All Outcomes</option>
        <option value="LONG">Long</option>
        <option value="SHORT">Short</option>
        <option value="WAIT">Wait</option>
      </select>
    </div>

    <div class="flex-1 overflow-y-auto">
      <table class="w-full text-left text-xs text-gray-300">
        <thead class="sticky top-0 bg-[#0E1217] text-gray-400">
          <tr>
            <th class="p-2 border-b border-white/10">Time</th>
            <th class="p-2 border-b border-white/10">Symbol</th>
            <th class="p-2 border-b border-white/10">Action</th>
            <th class="p-2 border-b border-white/10">Conf</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!filteredHistory.length">
            <td colspan="4" class="p-4 text-center text-gray-500">No records found</td>
          </tr>
          <tr v-for="(row, i) in filteredHistory" :key="i" class="border-b border-white/5 hover:bg-white/5">
            <td class="p-2 whitespace-nowrap">{{ row.timestamp?.split(' ')[1] || '--:--' }}</td>
            <td class="p-2 font-bold">{{ row.symbol }}</td>
            <td class="p-2">
              <span :class="row.action === 'LONG' ? 'text-accent-green' : row.action === 'SHORT' ? 'text-accent-red' : 'text-gray-400'">
                {{ row.action }}
              </span>
            </td>
            <td class="p-2">{{ row.confidence?.toFixed(0) || 0 }}%</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useTradingStore } from '../../stores/tradingStore'

const store = useTradingStore()

const filterSymbol = ref('all')
const filterAction = ref('all')

const symbols = computed(() => {
  const set = new Set<string>()
  store.decision_history.forEach((d: any) => {
    if (d.symbol) set.add(d.symbol)
  })
  return Array.from(set)
})

const filteredHistory = computed(() => {
  return store.decision_history.filter((d: any) => {
    if (filterSymbol.value !== 'all' && d.symbol !== filterSymbol.value) return false
    if (filterAction.value !== 'all' && !d.action?.toUpperCase().includes(filterAction.value)) return false
    return true
  }).slice(0, 100) // limit to 100
})
</script>
