<template>
  <button
    v-if="record.__expanded || Number(record.quantity ?? 0) > 1"
    type="button"
    class="qty-toggle"
    :class="{ 'qty-badge': isQtyBadge }"
    :style="isQtyBadge ? qtyBadgeStyle : undefined"
    @click.stop="emit('toggle', record)"
  >
    <span v-if="isQtyBadge" class="qty-badge-inner">{{ record.__expanded ? 1 : record.quantity }}</span>
    <template v-else>{{ record.quantity }}</template>
  </button>
  <span v-else>{{ record.quantity ?? 0 }}</span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { DisplayPrize } from './prizeTypes'

const props = defineProps<{ record: DisplayPrize }>()
const emit = defineEmits<{ toggle: [prize: DisplayPrize] }>()

const qtyBadgePalette = [
  { background: '#f5c542', border: '#d4a017' },
  { background: '#1677ff', border: '#0958d9' },
  { background: '#f5222d', border: '#cf1322' },
  { background: '#722ed1', border: '#531dab' },
  { background: '#fa8c16', border: '#d46b08' },
  { background: '#52c41a', border: '#389e0d' },
  { background: '#a8071a', border: '#820014' },
  { background: '#1f1f1f', border: '#000000' },
]

const isQtyBadge = computed(() => Number(props.record.quantity ?? 0) > 1)
const qtyBadgeStyle = computed(() => {
  const color = qtyBadgePalette[Math.max(0, props.record.__badgeIndex) % qtyBadgePalette.length]!
  return { background: color.background, borderColor: color.border }
})
</script>

<style scoped>
.qty-toggle {
  padding: 0;
  color: inherit;
  font: inherit;
  background: transparent;
  border: none;
  border-bottom: 1px solid currentColor;
  cursor: pointer;
}

.qty-toggle.qty-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  padding: 0;
  line-height: 1;
  border: 1px solid transparent;
  border-radius: 50%;
  box-shadow: inset -2px -3px 5px rgba(0, 0, 0, 0.22), inset 2px 2px 4px rgba(255, 255, 255, 0.3);
}

.qty-badge-inner {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  color: #303133;
  font-size: 12px;
  font-weight: 600;
  line-height: 1;
  background: #fff;
  border-radius: 50%;
}
</style>
