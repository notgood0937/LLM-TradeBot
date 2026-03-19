<template>
  <div class="glass-panel p-4 h-full flex flex-col">
    <h2 class="text-sm font-bold text-gray-400 mb-4 uppercase tracking-wider border-b border-white/10 pb-2">Trade Records</h2>
    <div class="flex-1 overflow-y-auto">
      <table class="w-full text-left text-xs text-gray-300">
        <thead class="sticky top-0 bg-[#0E1217] text-gray-400">
          <tr>
            <th class="p-2 border-b border-white/10">Time</th>
            <th class="p-2 border-b border-white/10">Symbol</th>
            <th class="p-2 border-b border-white/10">Side</th>
            <th class="p-2 border-b border-white/10">PnL</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!store.account.trade_history?.length">
            <td colspan="4" class="p-4 text-center text-gray-500">No trades executed</td>
          </tr>
          <tr v-for="(trade, i) in store.account.trade_history" :key="i" class="border-b border-white/5 hover:bg-white/5">
            <td class="p-2 whitespace-nowrap">{{ trade.timestamp?.split(' ')[1] || '--:--' }}</td>
            <td class="p-2 font-bold">{{ trade.symbol }}</td>
            <td class="p-2">
              <span :class="trade.side === 'LONG' || trade.side === 'BUY' ? 'text-accent-green' : trade.side === 'SHORT' || trade.side === 'SELL' ? 'text-accent-red' : 'text-gray-400'">
                {{ trade.side }}
              </span>
            </td>
            <td class="p-2" :class="trade.realized_pnl >= 0 ? 'text-accent-green' : 'text-accent-red'">
              {{ trade.realized_pnl >= 0 ? '+' : '' }}{{ trade.realized_pnl?.toFixed(2) }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useTradingStore } from '../../stores/tradingStore'
const store = useTradingStore()
</script>
