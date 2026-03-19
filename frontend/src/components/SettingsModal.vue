<template>
  <div v-if="isOpen" class="fixed inset-0 z-[100] flex items-center justify-center bg-black/80 backdrop-blur-sm">
    <div class="glass-panel w-full max-w-2xl max-h-[90vh] flex flex-col overflow-hidden shadow-2xl shadow-nofx-gold/20">
      <div class="p-4 border-b border-white/10 flex justify-between items-center bg-black/40">
        <h3 class="font-bold text-lg text-nofx-gold">⚙️ Settings</h3>
        <button @click="$emit('close')" class="text-gray-400 hover:text-white text-xl leading-none">&times;</button>
      </div>
      
      <div class="p-6 overflow-y-auto space-y-6 flex-1 text-sm bg-[#0E1217]">
        <!-- Run Mode -->
        <div class="space-y-2">
          <label class="font-bold text-gray-300">Trading Mode</label>
          <select v-model="form.trading.run_mode" class="w-full bg-black/50 border border-white/10 rounded p-2 focus:border-nofx-gold">
            <option value="test">Test Mode (Paper Trading)</option>
            <option value="live">Live Trading (Real Money)</option>
          </select>
          <p class="text-xs text-gray-500">Requires restart to fully apply connection changes.</p>
        </div>
        
        <!-- LLM Provider -->
        <div class="space-y-2">
          <label class="font-bold text-gray-300">🤖 LLM Provider</label>
          <select v-model="form.llm.llm_provider" class="w-full bg-black/50 border border-accent-green/50 rounded p-2 focus:border-accent-green">
            <option value="none">None (No LLM)</option>
            <option value="deepseek">DeepSeek (Default)</option>
            <option value="openai">OpenAI</option>
            <option value="claude">Claude</option>
            <option value="qwen">Qwen</option>
          </select>
        </div>

        <!-- API Keys (simplified for Vue port) -->
        <div class="space-y-4 pt-4 border-t border-white/10">
          <h4 class="font-bold text-gray-300">API Keys Configuration</h4>
          
          <div class="space-y-2">
            <label class="text-xs text-gray-400">Binance API Key</label>
            <input v-model="form.api_keys.binance_api_key" type="password" placeholder="Saved (Hidden)" class="w-full bg-black/50 border border-white/10 rounded p-2 focus:border-nofx-gold">
          </div>
          <div class="space-y-2">
            <label class="text-xs text-gray-400">Binance Secret Key</label>
            <input v-model="form.api_keys.binance_secret_key" type="password" placeholder="Saved (Hidden)" class="w-full bg-black/50 border border-white/10 rounded p-2 focus:border-nofx-gold">
          </div>
          <div class="space-y-2" v-if="form.llm.llm_provider === 'deepseek'">
            <label class="text-xs text-gray-400">DeepSeek API Key</label>
            <input v-model="form.api_keys.deepseek_api_key" type="password" placeholder="Saved (Hidden)" class="w-full bg-black/50 border border-white/10 rounded p-2 focus:border-nofx-gold">
          </div>
        </div>

        <!-- Quant Strategy -->
        <div class="space-y-4 pt-4 border-t border-white/10">
          <h4 class="font-bold text-gray-300">📈 Strategy Tuning</h4>
          <p class="text-xs text-gray-500">Fine-tune the indicator constraints used by the Quant Analyst agent.</p>
          
          <div class="grid grid-cols-2 gap-4">
            <div class="space-y-2">
              <label class="text-xs text-gray-400">EMA Short</label>
              <input v-model.number="form.strategy.ema_short_period" type="number" class="w-full bg-black/50 border border-white/10 rounded p-2 focus:border-nofx-gold">
            </div>
            <div class="space-y-2">
              <label class="text-xs text-gray-400">EMA Long</label>
              <input v-model.number="form.strategy.ema_long_period" type="number" class="w-full bg-black/50 border border-white/10 rounded p-2 focus:border-nofx-gold">
            </div>
            
            <div class="space-y-2">
              <label class="text-xs text-gray-400">RSI Period</label>
              <input v-model.number="form.strategy.rsi_period" type="number" class="w-full bg-black/50 border border-white/10 rounded p-2 focus:border-nofx-gold">
            </div>
            <div class="space-y-2">
              <label class="text-xs text-gray-400">RSI Boundaries (OB / OS)</label>
              <div class="flex gap-2">
                <input v-model.number="form.strategy.rsi_overbought" type="number" class="w-1/2 bg-black/50 border border-t-2 border-t-accent-red/50 rounded p-2 text-accent-red" title="Overbought">
                <input v-model.number="form.strategy.rsi_oversold" type="number" class="w-1/2 bg-black/50 border border-b-2 border-b-accent-green/50 rounded p-2 text-accent-green" title="Oversold">
              </div>
            </div>

            <div class="space-y-2">
              <label class="text-xs text-gray-400">KDJ Period</label>
              <input v-model.number="form.strategy.kdj_period" type="number" class="w-full bg-black/50 border border-white/10 rounded p-2 focus:border-nofx-gold">
            </div>
            <div class="space-y-2">
              <label class="text-xs text-gray-400">KDJ Boundaries (OB / OS)</label>
              <div class="flex gap-2">
                <input v-model.number="form.strategy.kdj_overbought" type="number" class="w-1/2 bg-black/50 border border-t-2 border-t-accent-red/50 rounded p-2 text-accent-red" title="Overbought">
                <input v-model.number="form.strategy.kdj_oversold" type="number" class="w-1/2 bg-black/50 border border-b-2 border-b-accent-green/50 rounded p-2 text-accent-green" title="Oversold">
              </div>
            </div>
          </div>
        </div>
      </div>
      
      <div class="p-4 border-t border-white/10 bg-black/40 flex justify-end gap-3">
        <button @click="$emit('close')" class="px-4 py-2 rounded border border-white/10 hover:bg-white/5 disabled:opacity-50">Cancel</button>
        <button @click="save" :disabled="saving" class="px-4 py-2 rounded bg-nofx-gold text-black font-bold hover:bg-nofx-gold-highlight disabled:opacity-50">
          {{ saving ? 'Saving...' : 'Save Changes' }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import axios from 'axios'

defineProps<{ isOpen: boolean }>()
const emit = defineEmits(['close'])

const saving = ref(false)
const form = ref({
  trading: { run_mode: 'test' },
  llm: { llm_provider: 'deepseek' },
  api_keys: {
    binance_api_key: '',
    binance_secret_key: '',
    deepseek_api_key: ''
  },
  strategy: {
    ema_short_period: 20,
    ema_long_period: 60,
    rsi_period: 14,
    rsi_overbought: 70,
    rsi_oversold: 30,
    kdj_period: 9,
    kdj_overbought: 80,
    kdj_oversold: 20
  }
})

onMounted(async () => {
  try {
    const res = await axios.get('/api/config')
    if (res.data) {
      if (res.data.trading?.run_mode) form.value.trading.run_mode = res.data.trading.run_mode
      if (res.data.llm?.provider) form.value.llm.llm_provider = res.data.llm.provider
      if (res.data.strategy) Object.assign(form.value.strategy, res.data.strategy)
    }
  } catch(e) { console.error('Failed to load settings', e) }
})

async function save() {
  saving.value = true
  try {
    await axios.post('/api/config', form.value)
    emit('close')
  } catch(e) {
    console.error('Failed to save settings', e)
    alert('Failed to save settings')
  } finally {
    saving.value = false
  }
}
</script>
