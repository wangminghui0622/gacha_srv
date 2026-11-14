<template>
  <PrizeTableBase
    :data="data"
    :probability="probability"
    :custom-row="customRow"
    :actions-width="150"
    @toggle="emit('toggle', $event)"
  >
    <template #name="{ record }">
      <a-tag v-if="record.hasParent" color="blue" style="margin-right: 4px">子赏</a-tag>
      {{ record.name }}
    </template>
    <template #actions="{ record }">
      <a-button
        class="prize-action-link"
        type="link"
        size="small"
        :disabled="!!record.hasParent"
        @click.stop="emit('child', record)"
      >子赏</a-button>
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
}>()

const emit = defineEmits<{
  toggle: [prize: DisplayPrize]
  remove: [prize: DisplayPrize]
  child: [prize: DisplayPrize]
}>()
</script>

<style scoped>
.prize-action-link {
  font-size: 17px !important;
}
</style>
