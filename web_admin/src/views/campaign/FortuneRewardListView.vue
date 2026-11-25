<template>
  <div>
    <div class="fortune-list-page">
      <div class="list-toolbar">
        <a-button type="primary" :loading="loading" @click="queryList">查询</a-button>
        <a-button v-if="canWrite" @click="goCreate">创建</a-button>
      </div>

      <a-table
        :data-source="records"
        :loading="loading"
        :pagination="false"
        :row-key="(record: FortuneReward) => String(record.id)"
        :scroll="{ x: 1500 }"
        :expand-column-width="48"
        size="middle"
        class="fortune-table"
      >
        <a-table-column title="ID" width="80" align="center">
          <template #default="{ record }">{{ record.id }}</template>
        </a-table-column>
        <a-table-column title="名称" min-width="280">
          <template #default="{ record }">{{ record.name || '-' }}</template>
        </a-table-column>
        <a-table-column title="兑换积分" width="110" align="center">
          <template #default="{ record }">{{ record.exchangePoints ?? 0 }}</template>
        </a-table-column>
        <a-table-column title="直购金额" width="110" align="center">
          <template #default="{ record }">{{ amountText(record.directBuyMinor) }}</template>
        </a-table-column>
        <a-table-column title="库存" width="90" align="center">
          <template #default="{ record }">{{ record.stock ?? 0 }}</template>
        </a-table-column>
        <a-table-column title="开始时间" width="180" align="center">
          <template #default="{ record }">{{ timeText(record.startAtUnixMillis) }}</template>
        </a-table-column>
        <a-table-column title="结束时间" width="180" align="center">
          <template #default="{ record }">{{ timeText(record.endAtUnixMillis) }}</template>
        </a-table-column>
        <a-table-column title="返现" width="90" align="center">
          <template #default="{ record }">{{ amountText(record.cashbackMinor) }}</template>
        </a-table-column>
        <a-table-column title="展示图" width="100" align="center">
          <template #default="{ record }">
            <a-image
              v-if="record.coverUrl"
              :src="record.coverUrl"
              :width="44"
              :height="44"
              class="cover-image"
            />
            <span v-else>-</span>
          </template>
        </a-table-column>
        <a-table-column title="标签" width="100" align="center">
          <template #default="{ record }">
            <a-tag v-if="record.tag" color="blue">{{ record.tag }}</a-tag>
            <span v-else>-</span>
          </template>
        </a-table-column>
        <a-table-column title="操作" width="170" align="center" fixed="right">
          <template #default="{ record }">
            <a-space v-if="canWrite">
              <a-button size="small" @click="editRecord(record)">修改</a-button>
              <a-button
                v-if="record.enabled"
                size="small"
                danger
                :loading="switchingId === String(record.id)"
                @click="toggleStatus(record)"
              >
                关闭
              </a-button>
              <a-button
                v-else
                size="small"
                type="primary"
                :loading="switchingId === String(record.id)"
                @click="toggleStatus(record)"
              >
                开启
              </a-button>
            </a-space>
            <span v-else>-</span>
          </template>
        </a-table-column>

        <template #expandedRowRender="{ record }">
          <div class="reward-detail">
            <div class="reward-detail-title">奖励明细</div>
            <a-table
              :data-source="record.rewards ?? []"
              :pagination="false"
              :row-key="(_: FortuneRewardItem, index: number) => String(index)"
              :scroll="{ x: 760 }"
              size="small"
              class="reward-detail-table"
            >
              <a-table-column title="类型" width="110" align="center">
                <template #default="{ record: item }">{{ typeText(item.type) }}</template>
              </a-table-column>
              <a-table-column title="名称" min-width="220">
                <template #default="{ record: item }">{{ item.name || '-' }}</template>
              </a-table-column>
              <a-table-column title="值" min-width="160" align="center">
                <template #default="{ record: item }">{{ item.value || '-' }}</template>
              </a-table-column>
              <a-table-column title="数量" width="90" align="center">
                <template #default="{ record: item }">{{ item.quantity ?? 1 }}</template>
              </a-table-column>
              <a-table-column title="优惠券金额" width="120" align="center">
                <template #default="{ record: item }">{{ couponAmountText(item) }}</template>
              </a-table-column>
              <a-table-column title="缩略图" width="90" align="center">
                <template #default="{ record: item }">
                  <a-image
                    v-if="item.thumbnail"
                    :src="item.thumbnail"
                    :width="40"
                    :height="40"
                    class="reward-image"
                  />
                  <span v-else>-</span>
                </template>
              </a-table-column>
            </a-table>
          </div>
        </template>
      </a-table>

      <div class="list-footer">
        <a-pagination
          v-model:current="page"
          v-model:page-size="pageSize"
          :total="total"
          :page-size-options="['10', '20', '50', '100']"
          show-size-changer
          show-less-items
          @change="load"
        />
      </div>
    </div>

    <a-modal
      v-model:open="editorOpen"
      title="设置吉祥值奖励"
      :footer="null"
      :width="1120"
      :style="{ maxWidth: '96vw' }"
      :body-style="{ maxHeight: 'calc(100vh - 200px)', overflowY: 'auto', padding: '8px 24px 12px' }"
      :mask-closable="false"
      centered
      destroy-on-close
    >
      <FortuneRewardForm :key="editorKey" :record="editingRecord" @success="onEditorSuccess" />
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { message } from 'ant-design-vue'
import dayjs from 'dayjs'
import FortuneRewardForm from '@/views/campaign/FortuneRewardForm.vue'
import { fortuneRewardClient } from '@/api/fortune'
import type { FortuneReward, FortuneRewardItem } from '@/api/fortune'
import { allows } from '@/access/grants'
import { useSessionStore } from '@/stores/session'
import { newRequestId } from '@/utils/requestId'

const session = useSessionStore()
const canWrite = computed(() => allows(session.permissionNodes, 'campaign.definition', 'write'))

const loading = ref(false)
const records = ref<FortuneReward[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const switchingId = ref('')
const editorOpen = ref(false)
const editingRecord = ref<FortuneReward | null>(null)
const editorKey = ref(0)

function amountText(value: string | number | undefined) {
  const amount = Number(value ?? 0) / 100
  if (!Number.isFinite(amount)) return '0'
  return Number.isInteger(amount) ? String(amount) : amount.toFixed(2)
}

function timeText(value: string | number | undefined) {
  const ms = Number(value ?? 0)
  return ms > 0 ? dayjs(ms).format('YYYY-MM-DD HH:mm:ss') : '-'
}

function typeText(type: string | undefined) {
  switch (type) {
    case 'FORTUNE_REWARD_ITEM_TYPE_FORTUNE':
      return '吉祥值'
    case 'FORTUNE_REWARD_ITEM_TYPE_POINTS':
      return '积分'
    case 'FORTUNE_REWARD_ITEM_TYPE_COUPON':
      return '优惠券'
    case 'FORTUNE_REWARD_ITEM_TYPE_ITEM':
      return '物品'
    case 'FORTUNE_REWARD_ITEM_TYPE_BOX':
      return '箱子'
    case 'FORTUNE_REWARD_ITEM_TYPE_MATCH_APPEARANCE':
      return '对对碰外观'
    default:
      return '-'
  }
}

function couponAmountText(item: FortuneRewardItem) {
  if (item.type !== 'FORTUNE_REWARD_ITEM_TYPE_COUPON') return '-'
  return `¥${(Number(item.customAmountMinor ?? 0) / 100).toFixed(2)}`
}

async function load() {
  loading.value = true
  try {
    const result = await fortuneRewardClient().List({
      tenantId: session.tenantId ?? 1,
      page: page.value,
      pageSize: pageSize.value,
    })
    records.value = result.items ?? []
    total.value = Number(result.total ?? 0)
  } catch {
    records.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function queryList() {
  page.value = 1
  load()
}

function goCreate() {
  editingRecord.value = null
  editorKey.value += 1
  editorOpen.value = true
}

function editRecord(record: FortuneReward) {
  editingRecord.value = record
  editorKey.value += 1
  editorOpen.value = true
}

function onEditorSuccess() {
  editorOpen.value = false
  load()
}

function updatePayload(record: FortuneReward, enabled: boolean) {
  return {
    name: record.name ?? '',
    exchangePoints: record.exchangePoints ?? 0,
    directBuyMinor: record.directBuyMinor ?? 0,
    stock: record.stock ?? 0,
    startAtUnixMillis: record.startAtUnixMillis ?? 0,
    endAtUnixMillis: record.endAtUnixMillis ?? 0,
    cashbackMinor: record.cashbackMinor ?? 0,
    rewards: (record.rewards ?? []).map((item) => ({
      type: item.type,
      value: item.value ?? '',
      quantity: item.quantity ?? 1,
      customAmountMinor: item.customAmountMinor ?? 0,
      name: item.name ?? '',
      thumbnail: item.thumbnail ?? '',
      extra: item.extra ?? '',
    })),
    coverUrl: record.coverUrl ?? '',
    tag: record.tag ?? '',
    enabled,
  }
}

async function toggleStatus(record: FortuneReward) {
  if (!record.id) return
  const id = String(record.id)
  const enabled = !record.enabled
  switchingId.value = id
  try {
    await fortuneRewardClient(newRequestId()).Update(record.id, updatePayload(record, enabled))
    record.enabled = enabled
    message.success(enabled ? '已开启' : '已关闭')
  } catch {
    // 错误提示由请求层统一处理
  } finally {
    switchingId.value = ''
  }
}

onMounted(load)
</script>

<style scoped lang="scss">
.fortune-list-page {
  padding: 20px 24px 28px;
  background: #fff;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
}
.list-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}
.list-toolbar :deep(.ant-btn) {
  min-width: 72px;
}
.fortune-table :deep(.ant-table-thead > tr > th) {
  background: #fafafa;
  color: #303133;
  font-weight: 600;
}
.fortune-table :deep(.ant-table-tbody > tr > td) {
  padding-top: 14px;
  padding-bottom: 14px;
}
.fortune-table :deep(.ant-table-row-expand-icon) {
  border-color: #d9d9d9;
}
.cover-image,
.reward-image {
  object-fit: contain;
  background: #fff;
  border-radius: 4px;
}
.reward-detail {
  padding: 8px 12px 12px;
  background: #fafafa;
  border-radius: 4px;
}
.reward-detail-title {
  margin-bottom: 8px;
  font-size: 14px;
  font-weight: 600;
  color: #303133;
}
.reward-detail-table :deep(.ant-table) {
  background: #fff;
}
.list-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 20px;
}
</style>
