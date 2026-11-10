<template>
  <div class="security-watermark" aria-hidden="true">
    <div v-for="row in rows" :key="row" class="watermark-row">
      <span v-for="col in cols" :key="`${row}-${col}`" class="watermark-cell">{{ label }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useSessionStore } from '@/stores/session'

const session = useSessionStore()
const rows = 8
const cols = 4

const label = computed(() => {
  const actor = session.claims?.actor_id ?? '—'
  const tenant = session.claims?.tenant_id ?? '—'
  const tail = session.accessToken ? session.accessToken.slice(-8) : 'guest'
  return `LB ${tenant}/${actor} · ${tail}`
})
</script>

<style scoped lang="scss">
.security-watermark {
  pointer-events: none;
  position: fixed;
  inset: 0;
  z-index: 9999;
  overflow: hidden;
  opacity: 0.12;
  user-select: none;
}

.watermark-row {
  display: flex;
  justify-content: space-around;
  transform: rotate(-24deg);
  margin-top: 120px;
}

.watermark-cell {
  font-size: 14px;
  color: #111;
  white-space: nowrap;
  padding: 24px 48px;
}
</style>
