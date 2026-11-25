<template>
  <a-table
    :data-source="data"
    :pagination="false"
    :row-key="(record: DisplayPrize) => record.__rowKey"
    size="small"
    :locale="{ emptyText: '暂无数据' }"
    :scroll="{ x: showRecord || showQuantityInput ? 900 : showSpecial ? 880 : 680 }"
    :custom-row="customRow"
  >
    <a-table-column title="序号" width="50" align="center">
      <template #default="{ record }">{{ record.__displayIndex }}</template>
    </a-table-column>
    <a-table-column title="赏别" data-index="prizeGrade" width="80" align="center" />
    <a-table-column title="类型" width="70" align="center">
      <template #default="{ record }">{{ record.prizeType === 2 ? '商品' : record.prizeType === 3 ? '箱子' : '虚拟' }}</template>
    </a-table-column>
    <a-table-column v-if="showSpecial" title="特别赏" width="90" align="center">
      <template #default="{ record }">{{ specialKindText(record.specialKind) }}</template>
    </a-table-column>
    <a-table-column v-if="showReward" title="奖励" width="120" align="center">
      <template #default="{ record }">{{ rewardText(record) }}</template>
    </a-table-column>
    <a-table-column v-if="showTarget" title="目标碰数" width="110" align="center">
      <template #default="{ record }">
        <div class="target-cell" @click.stop @mousedown.stop>
          <a-input-number
            :value="record.targetValue ?? 0"
            :min="0"
            size="small"
            style="width: 88px"
            @change="(value: number | null) => emit('targetChange', record, value)"
          />
        </div>
      </template>
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
    <a-table-column v-if="showRecord" title="中奖记录" width="90" align="center">
      <template #default="{ record }">
        <a-switch
          :checked="record.isPrize !== false"
          size="small"
          @click.stop
          @change="(checked: boolean) => emit('recordChange', record, checked)"
        />
      </template>
    </a-table-column>
    <a-table-column title="成本价" width="80" align="center">
      <template #default="{ record }">{{ Number(record.costFen ?? 0) / 100 }}</template>
    </a-table-column>
    <a-table-column title="总数" width="90" align="center">
      <template #default="{ record }">
        <a-input-number
          v-if="showQuantityInput"
          :value="record.quantity ?? 1"
          :min="-1"
          :precision="0"
          size="small"
          :controls="false"
          style="width: 72px"
          @click.stop
          @change="(value: number | null) => emit('quantityChange', record, value)"
        />
        <PrizeQuantityCell v-else :record="record" :expandable="!showTarget" @toggle="emit('toggle', $event)" />
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
    showTarget?: boolean
    showSpecial?: boolean
    showReward?: boolean
    showRecord?: boolean
    showQuantityInput?: boolean
  }>(),
  { actionsWidth: 100, showTarget: false, showSpecial: false, showReward: false, showRecord: false, showQuantityInput: false },
)

const emit = defineEmits<{
  toggle: [prize: DisplayPrize]
  targetChange: [record: DisplayPrize, value: number | null]
  recordChange: [record: DisplayPrize, value: boolean]
  quantityChange: [record: DisplayPrize, value: number | null]
}>()

function specialKindText(value: string | undefined) {
  switch (value) {
    case 'LAST':
      return 'Last赏'
    case 'MVP':
      return 'MVP赏'
    case 'LUCKY':
      return '幸运赏'
    case 'SETTLEMENT':
      return '结算奖'
    default:
      return '普通'
  }
}

function rewardText(record: DisplayPrize) {
  if (Number(record.cashFen ?? 0) > 0) return `¥${(Number(record.cashFen) / 100).toFixed(2)}`
  if (Number(record.pointsAmount ?? 0) > 0) return `${record.pointsAmount} 积分`
  if (record.couponTemplateNo) return `券 ${record.couponTemplateNo}`
  return '-'
}
</script>

<style scoped>
:deep(.ant-table-tbody > tr.ant-table-row:hover > td),
:deep(.ant-table-tbody > tr.ant-table-row > td.ant-table-cell-row-hover) {
  background: #f0f0f0 !important;
}

.target-cell {
  margin: -8px;
  padding: 8px 0;
}
</style>
