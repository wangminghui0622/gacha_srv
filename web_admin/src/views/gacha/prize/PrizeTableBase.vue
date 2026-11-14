<template>
  <a-table
    :data-source="data"
    :pagination="false"
    :row-key="(record: DisplayPrize) => record.__rowKey"
    size="small"
    :locale="{ emptyText: '暂无数据' }"
    :scroll="{ x: 680 }"
    :custom-row="customRow"
  >
    <a-table-column title="序号" width="50" align="center">
      <template #default="{ record }">{{ record.__displayIndex }}</template>
    </a-table-column>
    <a-table-column title="赏别" data-index="prizeGrade" width="80" align="center" />
    <a-table-column title="类型" width="70" align="center">
      <template #default="{ record }">{{ record.prizeType === 2 ? '商品' : record.prizeType === 3 ? '箱子' : '虚拟' }}</template>
    </a-table-column>
    <a-table-column title="赏品" align="center">
      <template #default="{ record }">
        <slot name="name" :record="record">{{ record.name }}</slot>
      </template>
    </a-table-column>
    <a-table-column title="缩略图" :width="80" align="center">
      <template #default="{ record }">
        <a-image v-if="record.imageUrl" :src="record.imageUrl" :width="40" :height="40" :preview="false" />
        <span v-else>—</span>
      </template>
    </a-table-column>
    <a-table-column title="成本价" width="80" align="center">
      <template #default="{ record }">{{ Number(record.costFen ?? 0) / 100 }}</template>
    </a-table-column>
    <a-table-column title="总数" width="60" align="center">
      <template #default="{ record }">
        <PrizeQuantityCell :record="record" @toggle="emit('toggle', $event)" />
      </template>
    </a-table-column>
    <a-table-column title="概率(%)" width="80" align="center">
      <template #default="{ record }">{{ probability(record) }}</template>
    </a-table-column>
    <a-table-column title="操作" :width="actionsWidth" align="center">
      <template #default="{ record }">
        <template v-if="record.__copyIndex === 0">
          <slot name="actions" :record="record" />
        </template>
      </template>
    </a-table-column>
  </a-table>
</template>

<script setup lang="ts">
import PrizeQuantityCell from './PrizeQuantityCell.vue'
import type { DisplayPrize } from './prizeTypes'

withDefaults(
  defineProps<{
    data: DisplayPrize[]
    probability: (prize: DisplayPrize) => string
    customRow: (record: DisplayPrize) => Record<string, unknown>
    actionsWidth?: number
  }>(),
  { actionsWidth: 100 },
)

const emit = defineEmits<{ toggle: [prize: DisplayPrize] }>()
</script>

<style scoped>
:deep(.ant-table-tbody > tr.ant-table-row:hover > td),
:deep(.ant-table-tbody > tr.ant-table-row > td.ant-table-cell-row-hover) {
  background: #dbeafe !important;
}
</style>
