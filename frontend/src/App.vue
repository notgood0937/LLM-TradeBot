<template>
  <div :class="{ 'editorial-mode': !store.isCleanMode }">
    <div v-if="!store.isCleanMode" class="crt-overlay"></div>
    <RouterView />
  </div>
</template>

<script setup lang="ts">
import { RouterView } from 'vue-router'
import { useTradingStore } from './stores/tradingStore'

const store = useTradingStore()
</script>
