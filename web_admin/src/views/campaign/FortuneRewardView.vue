<template>
  <section class="fortune-page">
    <h2 class="fortune-title">设置吉祥值奖励</h2>

    <a-form class="fortune-form" :label-col="{ span: 6 }" :wrapper-col="{ span: 18 }">
      <a-form-item label="名称" required>
        <a-input v-model:value="form.name" :maxlength="128" style="width: 280px" />
      </a-form-item>
      <a-form-item label="兑换积分">
        <a-input-number v-model:value="form.exchangePoints" :min="0" :precision="0" style="width: 280px" />
      </a-form-item>
      <a-form-item label="直购金额">
        <a-input-number v-model:value="form.directBuyYuan" :min="0" :precision="2" style="width: 280px" />
      </a-form-item>
      <a-form-item label="库存">
        <a-input-number v-model:value="form.stock" :min="0" :precision="0" style="width: 280px" />
      </a-form-item>
      <a-form-item label="开始时间">
        <a-date-picker v-model:value="form.startAt" show-time format="YYYY-MM-DD HH:mm:ss" style="width: 280px" />
      </a-form-item>
      <a-form-item label="结束时间">
        <a-date-picker v-model:value="form.endAt" show-time format="YYYY-MM-DD HH:mm:ss" style="width: 280px" />
      </a-form-item>
      <a-form-item label="返现金额">
        <a-input-number v-model:value="form.cashbackYuan" :min="0" :precision="2" style="width: 280px" />
      </a-form-item>

      <a-form-item label="奖励" required>
        <div class="fortune-reward-builder">
          <div class="fortune-reward-line">
            <span class="fortune-reward-label">吉祥值：</span>
            <a-input-number v-model:value="builder.fortune" :min="0" :precision="0" style="width: 120px" />
            <a-button size="small" :disabled="builder.fortune <= 0" @click="addFortune">添加</a-button>
          </div>
          <div class="fortune-reward-line">
            <span class="fortune-reward-label">积分：</span>
            <a-input-number v-model:value="builder.points" :min="0" :precision="0" style="width: 120px" />
            <a-button size="small" :disabled="builder.points <= 0" @click="addPoints">添加</a-button>
          </div>
          <div class="fortune-reward-line">
            <span class="fortune-reward-label">优惠券：</span>
            <span class="fortune-inline-label">优惠券id:</span>
            <a-input-number v-model:value="builder.couponId" :min="0" :precision="0" style="width: 150px" />
            <span class="fortune-inline-label">自定义金额:</span>
            <a-input-number v-model:value="builder.couponCustomYuan" :min="0" :precision="2" style="width: 100px" />
            <span class="fortune-inline-label">数量:</span>
            <a-input-number v-model:value="builder.couponQuantity" :min="1" :precision="0" style="width: 80px" />
            <a-button size="small" :disabled="builder.couponId <= 0" @click="addCoupon">添加</a-button>
          </div>
          <div class="fortune-reward-line">
            <span class="fortune-reward-label">物品：</span>
            <span class="fortune-inline-label">物品id:</span>
            <a-input-number v-model:value="builder.itemId" :min="0" :precision="0" style="width: 200px" />
            <span class="fortune-inline-label">数量:</span>
            <a-input-number v-model:value="builder.itemQuantity" :min="1" :precision="0" style="width: 80px" />
            <a-button size="small" :disabled="builder.itemId <= 0" @click="addItem">添加</a-button>
          </div>
          <div class="fortune-reward-line">
            <span class="fortune-reward-label">箱子：</span>
            <span class="fortune-inline-label">箱子id:</span>
            <a-input v-model:value="builder.boxId" style="width: 200px" />
            <span class="fortune-inline-label">数量:</span>
            <a-input-number v-model:value="builder.boxQuantity" :min="1" :precision="0" style="width: 80px" />
            <a-button size="small" :disabled="!builder.boxId.trim()" @click="addBox">添加</a-button>
          </div>
          <div class="fortune-reward-line">
            <span class="fortune-reward-label">对对碰外观：</span>
            <span class="fortune-inline-label">外观id:</span>
            <a-input-number v-model:value="builder.appearanceId" :min="0" :precision="0" style="width: 150px" />
            <a-button size="small" :disabled="builder.appearanceId <= 0" @click="addAppearance">添加</a-button>
          </div>
        </div>
      </a-form-item>

      <a-form-item label=" ">
        <a-table
          :data-source="rewards"
          :pagination="false"
          :row-key="(record: RewardDraft) => record._key"
          :scroll="{ x: 900 }"
          size="small"
        >
          <a-table-column title="ID" width="70" align="center">
            <template #default="{ index }">{{ index }}</template>
          </a-table-column>
          <a-table-column title="类型" width="120" align="center">
            <template #default="{ record }">{{ typeText(record.type) }}</template>
          </a-table-column>
          <a-table-column title="值" min-width="180" align="center">
            <template #default="{ record }">{{ record.value || '-' }}</template>
          </a-table-column>
          <a-table-column title="名称" min-width="180" align="center">
            <template #default="{ record }">{{ record.name || '-' }}</template>
          </a-table-column>
          <a-table-column title="缩略图" width="110" align="center">
            <template #default="{ record }">
              <img v-if="record.thumbnail" :src="record.thumbnail" class="fortune-thumb" alt="" />
              <span v-else>-</span>
            </template>
          </a-table-column>
          <a-table-column title="补充参数" width="130" align="center">
            <template #default="{ record }">{{ extraText(record) }}</template>
          </a-table-column>
          <a-table-column title="数量" width="90" align="center">
            <template #default="{ record }">{{ record.quantity ?? 1 }}</template>
          </a-table-column>
          <a-table-column title="操作" width="100" align="center">
            <template #default="{ index }">
              <a-button danger size="small" @click="removeReward(index)">删除</a-button>
            </template>
          </a-table-column>
        </a-table>
      </a-form-item>

      <a-form-item label="封面大图">
        <a-upload :show-upload-list="false" accept="image/*" :before-upload="handleCoverUpload">
          <div class="fortune-cover">
            <img v-if="form.coverUrl" :src="form.coverUrl" alt="" />
            <div v-else class="fortune-cover-placeholder">
              <span class="fortune-plus">+</span>
              <span>Upload</span>
            </div>
          </div>
        </a-upload>
      </a-form-item>

      <a-form-item label="标签">
        <a-select v-model:value="form.tag" :options="tagOptions" allow-clear placeholder="请选择标签" style="width: 280px" />
      </a-form-item>

      <a-form-item label="是否开启">
        <a-switch v-model:checked="form.enabled" />
      </a-form-item>
    </a-form>

    <div class="fortune-actions">
      <a-button type="primary" :loading="saving" @click="submit">确定</a-button>
      <a-button style="margin-left: 12px" @click="resetForm">重置</a-button>
    </div>

  </section>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import dayjs from 'dayjs'
import type { Dayjs } from 'dayjs'
import { fortuneRewardClient } from '@/api/fortune'
import type { FortuneRewardItem } from '@/api/fortune'
import { promotionClient } from '@/api/promotion'
import type { CouponTemplate } from '@/api/promotion'
import { catalogAdminProductClient } from '@/api/catalog'
import { poolClient } from '@/api/pool'
import { gachaMatchClient } from '@/api/gachaMatch'
import { uploadFileToOss } from '@/api/upload'
import { useSessionStore } from '@/stores/session'
import { newRequestId } from '@/utils/requestId'

type RewardDraft = FortuneRewardItem & { _key: string }

const session = useSessionStore()
const saving = ref(false)
const couponTemplates = ref<CouponTemplate[]>([])

const tagOptions = ['新品', '热门', '限时', '推荐'].map((value) => ({ value, label: value }))

const form = reactive({
  name: '',
  exchangePoints: 0,
  directBuyYuan: 0,
  stock: 0,
  startAt: null as Dayjs | null,
  endAt: null as Dayjs | null,
  cashbackYuan: 0,
  coverUrl: '',
  tag: undefined as string | undefined,
  enabled: true,
})

const builder = reactive({
  fortune: 0,
  points: 0,
  couponId: 0,
  couponCustomYuan: 0,
  couponQuantity: 1,
  itemId: 0,
  itemQuantity: 1,
  boxId: '',
  boxQuantity: 1,
  appearanceId: 0,
})

const rewards = ref<RewardDraft[]>([])

function tenantID() {
  return String(session.tenantId ?? 1)
}

function yuanToFen(value: number) {
  return Math.max(0, Math.round(Number(value || 0) * 100))
}

function nextKey() {
  return `${Date.now()}-${Math.random()}`
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

function extraText(record: FortuneRewardItem) {
  if (record.type === 'FORTUNE_REWARD_ITEM_TYPE_COUPON') {
    return `¥${(Number(record.customAmountMinor ?? 0) / 100).toFixed(2)}`
  }
  return record.extra || '0'
}

function couponName(id: string) {
  const template = couponTemplates.value.find((item) => String(item.templateId ?? '') === id)
  return template?.name || `优惠券 #${id}`
}

function addFortune() {
  rewards.value.push({
    _key: nextKey(),
    type: 'FORTUNE_REWARD_ITEM_TYPE_FORTUNE',
    value: String(builder.fortune),
    quantity: 1,
    name: '吉祥值',
    extra: '0',
  })
  builder.fortune = 0
}

function addPoints() {
  rewards.value.push({
    _key: nextKey(),
    type: 'FORTUNE_REWARD_ITEM_TYPE_POINTS',
    value: String(builder.points),
    quantity: 1,
    name: '积分',
    extra: '0',
  })
  builder.points = 0
}

function addCoupon() {
  if (builder.couponId <= 0) return
  const id = String(builder.couponId)
  rewards.value.push({
    _key: nextKey(),
    type: 'FORTUNE_REWARD_ITEM_TYPE_COUPON',
    value: id,
    quantity: Math.max(1, Math.trunc(builder.couponQuantity)),
    customAmountMinor: yuanToFen(builder.couponCustomYuan),
    name: couponName(id),
    extra: '0',
  })
  builder.couponId = 0
  builder.couponCustomYuan = 0
  builder.couponQuantity = 1
}

async function addItem() {
  if (builder.itemId <= 0) return
  const id = String(builder.itemId)
  let name = `物品 #${id}`
  let thumbnail = ''
  try {
    const detail = await catalogAdminProductClient.Get(id)
    name = detail.product?.title || name
    thumbnail = detail.product?.coverUrl || detail.thumbUrl || ''
  } catch {
    // 查不到时保留 ID 展示
  }
  rewards.value.push({
    _key: nextKey(),
    type: 'FORTUNE_REWARD_ITEM_TYPE_ITEM',
    value: id,
    quantity: Math.max(1, Math.trunc(builder.itemQuantity)),
    name,
    thumbnail,
    extra: '0',
  })
  builder.itemId = 0
  builder.itemQuantity = 1
}

async function addBox() {
  const id = builder.boxId.trim()
  if (!id) return
  let name = `箱子 #${id}`
  try {
    const detail = await poolClient().GetBoxView({ boxId: id }, { silentError: true })
    name = detail?.boxName || name
  } catch {
    // 查不到时保留 ID 展示
  }
  rewards.value.push({
    _key: nextKey(),
    type: 'FORTUNE_REWARD_ITEM_TYPE_BOX',
    value: id,
    quantity: Math.max(1, Math.trunc(builder.boxQuantity)),
    name,
    extra: '0',
  })
  builder.boxId = ''
  builder.boxQuantity = 1
}

async function addAppearance() {
  if (builder.appearanceId <= 0) return
  const id = String(builder.appearanceId)
  let name = `外观 #${id}`
  let thumbnail = ''
  try {
    const result = await gachaMatchClient.ListAppearances({ pageSize: 200 })
    const found = result.items?.find((item) => String(item.id ?? '') === id)
    name = found?.name || name
    thumbnail = found?.imageUrl || ''
  } catch {
    // 查不到时保留 ID 展示
  }
  rewards.value.push({
    _key: nextKey(),
    type: 'FORTUNE_REWARD_ITEM_TYPE_MATCH_APPEARANCE',
    value: id,
    quantity: 1,
    name,
    thumbnail,
    extra: '0',
  })
  builder.appearanceId = 0
}

function removeReward(index: number) {
  rewards.value.splice(index, 1)
}

async function handleCoverUpload(file: File) {
  try {
    const result = await uploadFileToOss(file, 'fortune')
    if (result.url) form.coverUrl = result.url
    message.success('封面上传成功')
  } catch {
    message.error('封面上传失败')
  }
  return false
}

function resetForm() {
  form.name = ''
  form.exchangePoints = 0
  form.directBuyYuan = 0
  form.stock = 0
  form.startAt = null
  form.endAt = null
  form.cashbackYuan = 0
  form.coverUrl = ''
  form.tag = undefined
  form.enabled = true
  rewards.value = []
  builder.fortune = 0
  builder.points = 0
  builder.couponId = 0
  builder.couponCustomYuan = 0
  builder.couponQuantity = 1
  builder.itemId = 0
  builder.itemQuantity = 1
  builder.boxId = ''
  builder.boxQuantity = 1
  builder.appearanceId = 0
}

async function submit() {
  if (!form.name.trim()) {
    message.warning('请输入名称')
    return
  }
  if (rewards.value.length === 0) {
    message.warning('请至少添加一个奖励')
    return
  }
  if (form.startAt && form.endAt && form.endAt.valueOf() < form.startAt.valueOf()) {
    message.warning('结束时间不能早于开始时间')
    return
  }
  const payload = {
    name: form.name.trim(),
    exchangePoints: Math.max(0, Math.trunc(form.exchangePoints)),
    directBuyMinor: yuanToFen(form.directBuyYuan),
    stock: Math.max(0, Math.trunc(form.stock)),
    startAtUnixMillis: form.startAt ? form.startAt.valueOf() : 0,
    endAtUnixMillis: form.endAt ? form.endAt.valueOf() : 0,
    cashbackMinor: yuanToFen(form.cashbackYuan),
    rewards: rewards.value.map(({ _key, ...rest }) => rest),
    coverUrl: form.coverUrl,
    tag: form.tag || '',
    enabled: form.enabled,
  }
  saving.value = true
  try {
    await fortuneRewardClient(newRequestId()).Create(payload)
    message.success('保存成功')
    resetForm()
  } catch {
    // 错误提示由请求层统一处理
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
  try {
    const result = await promotionClient().ListCouponTemplates({ tenantId: tenantID(), pageSize: 100 })
    couponTemplates.value = result.templates ?? []
  } catch {
    couponTemplates.value = []
  }
})
</script>

<style scoped lang="scss">
.fortune-page {
  padding: 20px 24px;
  background: #fff;
  min-height: calc(100vh - 128px);
}
.fortune-title {
  margin: 0 0 24px;
  font-size: 18px;
  font-weight: 600;
}
.fortune-form :deep(.ant-form-item-label) {
  width: 130px;
  flex: 0 0 130px;
  text-align: right;
}
.fortune-form :deep(.ant-form-item-label > label) {
  font-size: 15px;
}
.fortune-form :deep(.ant-form-item) {
  margin-bottom: 18px;
}
.fortune-reward-builder {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.fortune-reward-line {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.fortune-reward-label {
  width: 110px;
  text-align: right;
  font-size: 15px;
}
.fortune-inline-label {
  font-size: 15px;
  color: #303133;
}
.fortune-thumb {
  width: 48px;
  height: 48px;
  object-fit: cover;
  border-radius: 4px;
}
.fortune-cover {
  width: 90px;
  height: 90px;
  border: 1px dashed #d9d9d9;
  border-radius: 6px;
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #fafafa;
  cursor: pointer;
}
.fortune-cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.fortune-cover-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  color: #909399;
  font-size: 13px;
}
.fortune-plus {
  font-size: 24px;
  line-height: 1;
}
.fortune-actions {
  margin-left: 130px;
  margin-top: 8px;
}
</style>
