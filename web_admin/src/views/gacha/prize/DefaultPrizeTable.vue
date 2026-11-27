<template>
  <PrizeTableBase
    :data="data"
    :probability="probability"
    :custom-row="customRow"
    :show-target="showTarget"
    :show-special="showSpecial"
    :show-reward="showReward"
    :show-record="showRecord"
    :show-quantity-input="showQuantityInput"
    :show-lucky-stage="showLuckyStage"
    :actions-width="100"
    @toggle="emit('toggle', $event)"
    @target-change="(record, value) => emit('targetChange', record, value)"
    @record-change="(record, value) => emit('recordChange', record, value)"
    @quantity-change="(record, value) => emit('quantityChange', record, value)"
  >
    <template #actions="{ record }">
      <a-button class="prize-action-link" type="link" danger size="small" @click.stop="emit('remove', record)">删除</a-button>
    </template>
  </PrizeTableBase>
</template>

<script setup lang="ts">
import PrizeTableBase from './PrizeTableBase.vue'
import type { DisplayPrize } from './prizeTypes'

defineProps<{
  data: DisplayPrize[]
  probability: (prize: DisplayPrize) => string
  customRow: (record: DisplayPrize) => Record<string, unknown>
  showTarget?: boolean
  showSpecial?: boolean
  showReward?: boolean
  showRecord?: boolean
  showQuantityInput?: boolean
  showLuckyStage?: boolean
}>()

const emit = defineEmits<{
  toggle: [prize: DisplayPrize]
  remove: [prize: DisplayPrize]
  targetChange: [record: DisplayPrize, value: number | null]
  recordChange: [record: DisplayPrize, value: boolean]
  quantityChange: [record: DisplayPrize, value: number | null]
}>()
</script>

<style scoped>
.prize-action-link {
  font-size: 17px !important;
}
</style>
