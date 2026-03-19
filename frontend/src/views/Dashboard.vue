<template>
  <div class="container mx-auto p-4 flex flex-col gap-4 min-h-screen">
    <header class="glass-panel p-4 flex flex-wrap items-center justify-between gap-4">
      <div class="flex items-center gap-4">
        <div>
          <h1 class="text-xl font-bold text-nofx-gold text-glow">LLM-TradeBot Studio</h1>
          <p class="text-xs text-gray-500">Editorial command surface</p>
        </div>
      </div>
      
      <div class="flex flex-wrap items-center gap-4">
        <select v-model="store.system.cycle_interval" @change="store.triggerControl('set_interval')" class="bg-white/5 border border-white/10 rounded px-2 py-1 text-sm focus:border-nofx-gold cursor-pointer">
          <option :value="1">1 Min</option>
          <option :value="3">3 Min</option>
          <option :value="5">5 Min</option>
          <option :value="10">10 Min</option>
          <option :value="15">15 Min</option>
          <option :value="30">30 Min</option>
          <option :value="60">1 Hour</option>
          <option :value="240">4 Hours</option>
        </select>
        <button @click="store.triggerControl('start')" 
                :class="['tech-btn', store.system.running ? 'border-accent-green text-accent-green bg-accent-green/10 shadow-[0_4px_20px_rgba(14,203,129,0.2)]' : 'hover:border-accent-green hover:text-accent-green']">
          {{ store.system.running ? '▶ RUNNING' : '▶ RUN' }}
        </button>
        <button @click="store.triggerControl('pause')" 
                :class="['tech-btn', !store.system.running ? 'border-nofx-gold text-nofx-gold bg-nofx-gold/10 shadow-[0_4px_20px_rgba(240,185,11,0.2)]' : 'hover:border-nofx-gold hover:text-nofx-gold']">
          {{ !store.system.running ? '⏸ PAUSED' : '⏸ HOLD' }}
        </button>
        <button @click="showSettings = true" class="tech-btn ml-4">⚙️ Settings</button>
        <button @click="store.toggleCleanMode()" class="tech-btn">
          {{ store.isCleanMode ? 'CRT Mode' : 'Clean Mode' }}
        </button>
      </div>
    </header>

    <section class="grid grid-cols-2 lg:grid-cols-6 gap-4">
      <div class="glass-panel p-4">
        <span class="text-xs text-gray-400">Current Pair</span>
        <div class="font-bold text-lg text-white mt-1">{{ store.system.current_symbol }}</div>
      </div>
      <div class="glass-panel p-4">
        <span class="text-xs text-gray-400">System Mode</span>
        <div class="flex items-center gap-2 mt-1">
          <div class="font-bold text-lg" :class="store.system.is_test_mode ? 'text-purple-400' : 'text-accent-red'">
            {{ store.system.is_test_mode ? 'TEST' : 'LIVE' }}
          </div>
          <span class="text-[10px] px-2 py-0.5 rounded" :class="store.system.is_test_mode ? 'bg-purple-400/20 text-purple-400' : 'bg-accent-red/20 text-accent-red'">
            {{ store.system.is_test_mode ? '模拟盘 (Paper)' : '真实盘 (Real)' }}
          </span>
        </div>
      </div>
      <div class="glass-panel p-4">
        <span class="text-xs text-gray-400">Model Route</span>
        <div class="font-bold text-sm text-nofx-gold mt-1 uppercase">{{ store.llm_info.provider }} / {{ store.llm_info.model }}</div>
      </div>
      <div class="glass-panel p-4">
        <span class="text-xs text-gray-400">Equity</span>
        <div class="font-bold text-lg text-nofx-gold mt-1">{{ store.account.realtime_balance }}</div>
      </div>
      <div class="glass-panel p-4">
        <span class="text-xs text-gray-400">Win Rate</span>
        <div class="font-bold text-lg text-white mt-1">{{ store.account.win_rate }}</div>
      </div>
      <div class="glass-panel p-4">
        <span class="text-xs text-gray-400">Active Positions</span>
        <div class="font-bold text-lg text-white mt-1">{{ store.positions.length }}</div>
      </div>
    </section>

    <!-- Top Grid: Chart & LLM Feed -->
    <section class="grid grid-cols-1 lg:grid-cols-2 gap-4">
      <div class="h-[400px]"><EquityChart /></div>
      <div class="glass-panel p-4 flex flex-col items-stretch h-[400px]">
        <h2 class="text-sm font-bold text-gray-400 mb-4 uppercase tracking-wider border-b border-white/10 pb-2">Reasoning Feed</h2>
        <div class="flex-1 overflow-y-auto space-y-3 font-mono text-sm pr-2">
          <div v-if="!store.agents.agent_messages.length" class="text-gray-500">Initializing neural pathways...</div>
          <div v-for="(msg, i) in store.agents.agent_messages.slice().reverse()" :key="i" class="p-2 border border-white/5 rounded bg-black/30">
            <div class="flex justify-between text-xs text-gray-500 mb-1">
              <span>{{ msg.time }}</span>
              <span class="text-nofx-gold">{{ msg.agent }}</span>
            </div>
            <div class="text-gray-300 leading-relaxed">{{ msg.content }}</div>
          </div>
        </div>
      </div>
    </section>

    <!-- Bottom Grid: Tables & Logs -->
    <main class="grid grid-cols-1 lg:grid-cols-3 gap-4 flex-1">
      
      <aside class="flex flex-col gap-4 h-[400px]">
        <DecisionTable />
      </aside>

      <section class="flex flex-col gap-4 h-[400px]">
        <TradeTable />
      </section>

      <aside class="flex flex-col gap-4 h-[400px]">
        <div class="glass-panel p-4 flex-1 flex flex-col">
          <h2 class="text-sm font-bold text-gray-400 mb-4 uppercase tracking-wider border-b border-white/10 pb-2">Operator Log</h2>
          <div class="flex-1 overflow-y-auto font-mono text-xs space-y-1 text-gray-400">
             <div v-if="!store.logs.length">Awaiting logs...</div>
             <div v-for="(log, i) in store.logs" :key="i">{{ log }}</div>
          </div>
        </div>
      </aside>
    </main>
    
    <SettingsModal :isOpen="showSettings" @close="showSettings = false" />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useTradingStore } from '../stores/tradingStore'
import SettingsModal from '../components/SettingsModal.vue'
import EquityChart from '../components/chart/EquityChart.vue'
import DecisionTable from '../components/tables/DecisionTable.vue'
import TradeTable from '../components/tables/TradeTable.vue'

const store = useTradingStore()
const showSettings = ref(false)

onMounted(() => {
  store.startPolling()
})

onUnmounted(() => {
  store.stopPolling()
})
</script>
