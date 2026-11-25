<template>
  <div class="fortune-form-body">
    <div class="form-layout">
        <div class="form-left">
          <a-form :label-col="{ span: 8 }" :wrapper-col="{ span: 16 }" label-align="right">
            <a-form-item label="名称" required>
              <a-input v-model:value="form.name" :maxlength="128" placeholder="名称" />
            </a-form-item>
            <a-form-item label="兑换积分">
              <a-input-number v-model:value="form.exchangePoints" :min="0" :precision="0" style="width: 100%" />
            </a-form-item>
            <a-form-item label="直购金额">
              <a-input-number v-model:value="form.directBuyYuan" :min="0" :precision="2" style="width: 100%" />
            </a-form-item>
            <a-form-item label="库存">
              <a-input-number v-model:value="form.stock" :min="0" :precision="0" style="width: 100%" />
            </a-form-item>
            <a-form-item label="开始时间">
              <a-date-picker v-model:value="form.startAt" show-time format="YYYY-MM-DD HH:mm:ss" style="width: 100%" placeholder="请选择日期" />
            </a-form-item>
            <a-form-item label="结束时间">
              <a-date-picker v-model:value="form.endAt" show-time format="YYYY-MM-DD HH:mm:ss" style="width: 100%" placeholder="请选择日期" />
            </a-form-item>
            <a-form-item label="返现金额">
              <a-input-number v-model:value="form.cashbackYuan" :min="0" :precision="2" style="width: 100%" />
            </a-form-item>
            <a-form-item label="标签">
              <div class="tag-switch-row">
                <a-select
                  v-model:value="form.tag"
                  class="tag-select"
                  :options="tagOptions"
                  allow-clear
                  placeholder="请选择标签"
                />
                <span class="switch-label">是否开启</span>
                <a-switch v-model:checked="form.enabled" />
              </div>
            </a-form-item>
            <a-form-item label="封面大图">
              <div class="cover-upload-wrap">
                <a-upload :show-upload-list="false" accept="image/*" :before-upload="handleCoverUpload">
                  <div class="image-upload-box">
                    <img v-if="form.coverUrl" :src="form.coverUrl" alt="" />
                    <template v-else>
                      <span class="upload-plus">+</span>
                      <span>Upload</span>
                    </template>
                  </div>
                </a-upload>
                <button
                  v-if="form.coverUrl"
                  type="button"
                  class="cover-remove"
                  title="移除封面"
                  aria-label="移除封面"
                  @click.stop.prevent="removeCover"
                >
                  <CloseOutlined />
                </button>
              </div>
            </a-form-item>
          </a-form>
        </div>

        <div class="form-right">
          <div class="reward-section">
            <div class="section-title">奖励配置：</div>
            <div ref="panelRef" class="reward-builder-panel" :style="labelStyle">
              <div class="reward-line">
                <span class="inline-label reward-label-first">吉祥值：</span>
                <a-input-number v-model:value="builder.fortune" class="reward-input-first" :min="0" :precision="0" />
                <a-button :disabled="builder.fortune <= 0" @click="addFortune">添加</a-button>
              </div>
              <div class="reward-line">
                <span class="inline-label reward-label-first">积分：</span>
                <a-input-number v-model:value="builder.points" class="reward-input-first" :min="0" :precision="0" />
                <a-button :disabled="builder.points <= 0" @click="addPoints">添加</a-button>
              </div>
              <div class="reward-line">
                <span class="inline-label reward-label-first">优惠券：</span>
                <a-input-number v-model:value="builder.couponId" class="reward-input-first" :min="0" :precision="0" placeholder="优惠券id" />
                <span class="inline-label reward-label-second">自定义金额:</span>
                <a-input-number v-model:value="builder.couponCustomYuan" class="reward-input-second" :min="0" :precision="2" />
                <span class="inline-label reward-label-third">数量:</span>
                <a-input-number v-model:value="builder.couponQuantity" class="reward-input-third" :min="1" :precision="0" />
                <a-button :disabled="!builder.couponId" @click="addCoupon">添加</a-button>
              </div>
              <div class="reward-line">
                <span class="inline-label reward-label-first">物品：</span>
                <a-input-number v-model:value="builder.itemId" class="reward-input-first" :min="0" :precision="0" placeholder="物品id" />
                <span class="inline-label reward-label-second">数量:</span>
                <a-input-number v-model:value="builder.itemQuantity" class="reward-input-second" :min="1" :precision="0" />
                <a-button :disabled="!builder.itemId" @click="addItem">添加</a-button>
              </div>
              <div class="reward-line">
                <span class="inline-label reward-label-first">箱子：</span>
                <a-input v-model:value="builder.boxId" class="reward-input-first" placeholder="箱子id" />
                <span class="inline-label reward-label-second">数量:</span>
                <a-input-number v-model:value="builder.boxQuantity" class="reward-input-second" :min="1" :precision="0" />
                <a-button :disabled="!builder.boxId.trim()" @click="addBox">添加</a-button>
              </div>
              <div class="reward-line">
                <span class="inline-label reward-label-first">对对碰外观：</span>
                <a-input-number v-model:value="builder.appearanceId" class="reward-input-first" :min="0" :precision="0" placeholder="对对碰外观id" />
                <a-button :disabled="!builder.appearanceId" @click="addAppearance">添加</a-button>
              </div>
            </div>

            <a-table
              :data-source="rewards"
              :pagination="false"
              :row-key="(record: RewardDraft) => record._key"
              :scroll="{ x: 760 }"
              size="small"
              class="reward-table"
              :locale="{ emptyText: '暂无数据' }"
            >
              <a-table-column title="ID" width="70" align="center">
                <template #default="{ index }">{{ index }}</template>
              </a-table-column>
              <a-table-column title="类型" width="110" align="center">
                <template #default="{ record }">{{ typeText(record.type) }}</template>
              </a-table-column>
              <a-table-column title="值" min-width="160" align="center">
                <template #default="{ record }">{{ record.value || '-' }}</template>
              </a-table-column>
              <a-table-column title="名称" min-width="180" align="center">
                <template #default="{ record }">{{ record.name || '-' }}</template>
              </a-table-column>
              <a-table-column title="缩略图" width="90" align="center">
                <template #default="{ record }">
                  <img v-if="record.thumbnail" :src="record.thumbnail" class="reward-thumb" alt="" />
                  <span v-else>-</span>
                </template>
              </a-table-column>
              <a-table-column title="补充参数" width="110" align="center">
                <template #default="{ record }">{{ extraText(record) }}</template>
              </a-table-column>
              <a-table-column title="数量" width="80" align="center">
                <template #default="{ record }">{{ record.quantity ?? 1 }}</template>
              </a-table-column>
              <a-table-column title="操作" width="90" align="center">
                <template #default="{ index }">
                  <button
                    type="button"
                    class="reward-remove"
                    title="移除奖励"
                    aria-label="移除奖励"
                    @click="removeReward(index)"
                  >
                    <CloseOutlined />
                  </button>
                </template>
              </a-table-column>
            </a-table>
          </div>
        </div>
      </div>

    <div class="create-actions">
      <a-button class="btn-create" type="primary" :loading="saving" @click="submit">确定</a-button>
      <a-button @click="handleReset">重置</a-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'
import { message } from 'ant-design-vue'
import { CloseOutlined } from '@ant-design/icons-vue'
import dayjs from 'dayjs'
import type { Dayjs } from 'dayjs'
import { fortuneRewardClient } from '@/api/fortune'
import type { FortuneReward, FortuneRewardItem } from '@/api/fortune'
import { promotionClient } from '@/api/promotion'
import type { CouponTemplate } from '@/api/promotion'
import { catalogAdminProductClient } from '@/api/catalog'
import { poolClient } from '@/api/pool'
import { gachaMatchClient } from '@/api/gachaMatch'
import { uploadFileToOss } from '@/api/upload'
import { useSessionStore } from '@/stores/session'
import { newRequestId } from '@/utils/requestId'

type RewardDraft = FortuneRewardItem & { _key: string }

const props = defineProps<{ record?: FortuneReward | null }>()
const emit = defineEmits<{ (e: 'success'): void }>()

const session = useSessionStore()
const saving = ref(false)
const couponTemplates = ref<CouponTemplate[]>([])
const panelRef = ref<HTMLElement | null>(null)
const labelWidths = reactive({ first: 96, second: 80, third: 40 })
const labelStyle = computed(() => ({
  '--label-first-width': `${labelWidths.first}px`,
  '--label-second-width': `${labelWidths.second}px`,
  '--label-third-width': `${labelWidths.third}px`,
}))
const editingId = computed(() => props.record?.id ?? '')

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
  couponId: null as number | null,
  couponCustomYuan: 0,
  couponQuantity: 1,
  itemId: null as number | null,
  itemQuantity: 1,
  boxId: '',
  boxQuantity: 1,
  appearanceId: null as number | null,
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
  if (builder.couponId == null || builder.couponId <= 0) return
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
  builder.couponId = null
  builder.couponCustomYuan = 0
  builder.couponQuantity = 1
}

async function addItem() {
  if (builder.itemId == null || builder.itemId <= 0) return
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
  builder.itemId = null
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
  if (builder.appearanceId == null || builder.appearanceId <= 0) return
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
  builder.appearanceId = null
}

function removeReward(index: number) {
  rewards.value.splice(index, 1)
}

function removeCover() {
  form.coverUrl = ''
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
  builder.couponId = null
  builder.couponCustomYuan = 0
  builder.couponQuantity = 1
  builder.itemId = null
  builder.itemQuantity = 1
  builder.boxId = ''
  builder.boxQuantity = 1
  builder.appearanceId = null
}

function applyRecord(record: FortuneReward) {
  form.name = record.name ?? ''
  form.exchangePoints = Number(record.exchangePoints ?? 0)
  form.directBuyYuan = Number(record.directBuyMinor ?? 0) / 100
  form.stock = Number(record.stock ?? 0)
  const startAt = Number(record.startAtUnixMillis ?? 0)
  const endAt = Number(record.endAtUnixMillis ?? 0)
  form.startAt = startAt > 0 ? dayjs(startAt) : null
  form.endAt = endAt > 0 ? dayjs(endAt) : null
  form.cashbackYuan = Number(record.cashbackMinor ?? 0) / 100
  form.coverUrl = record.coverUrl ?? ''
  form.tag = record.tag || undefined
  form.enabled = record.enabled === true
  rewards.value = (record.rewards ?? []).map((item) => ({ ...item, _key: nextKey() }))
}

function handleReset() {
  if (props.record) {
    applyRecord(props.record)
  } else {
    resetForm()
  }
}

async function submit() {
  if (saving.value) return
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
    if (editingId.value) {
      await fortuneRewardClient(newRequestId()).Update(editingId.value, payload)
    } else {
      await fortuneRewardClient(newRequestId()).Create(payload)
    }
    message.success('保存成功')
    emit('success')
  } catch {
    // 错误提示由请求层统一处理
  } finally {
    saving.value = false
  }
}

watch(
  () => props.record,
  (value) => {
    if (value) {
      applyRecord(value)
    } else {
      resetForm()
    }
  },
  { immediate: true },
)

function measureLabelWidths() {
  const panel = panelRef.value
  if (!panel) return
  const measure = (selector: string, fallback: number) => {
    let max = 0
    panel.querySelectorAll<HTMLElement>(selector).forEach((el) => {
      const clone = el.cloneNode(true) as HTMLElement
      clone.style.position = 'absolute'
      clone.style.visibility = 'hidden'
      clone.style.pointerEvents = 'none'
      clone.style.whiteSpace = 'nowrap'
      clone.style.width = 'auto'
      clone.style.flex = '0 0 auto'
      clone.style.marginLeft = '0'
      clone.style.textAlign = 'left'
      clone.style.textAlignLast = 'auto'
      panel.appendChild(clone)
      max = Math.max(max, clone.getBoundingClientRect().width)
      clone.remove()
    })
    return max > 0 ? Math.ceil(max) + 2 : fallback
  }
  labelWidths.first = measure('.reward-label-first', 96)
  labelWidths.second = measure('.reward-label-second', 80)
  labelWidths.third = measure('.reward-label-third', 40)
}

onMounted(async () => {
  await nextTick()
  measureLabelWidths()
  void document.fonts.ready.then(() => measureLabelWidths())
  try {
    const result = await promotionClient().ListCouponTemplates({ tenantId: tenantID(), pageSize: 100 })
    couponTemplates.value = result.templates ?? []
  } catch {
    couponTemplates.value = []
  }
})
</script>

<style scoped lang="scss">
.fortune-form-body {
  padding-top: 4px;
}
.form-layout {
  display: grid;
  grid-template-columns: clamp(320px, 27%, 440px) minmax(0, 1fr);
  gap: 28px;
  align-items: start;
}
.form-left,
.form-right {
  min-width: 0;
}
.form-left :deep(.ant-form-item) {
  margin-bottom: 16px;
}
.reward-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-height: 100%;
}
.section-title {
  font-size: 15px;
  font-weight: 600;
  color: #303133;
}
.reward-builder-panel {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px 16px;
  background: #fafafa;
  border: 1px solid #f0f0f0;
  border-radius: 4px;
}
.reward-line {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}
.reward-builder-panel :deep(.ant-input-number-handler-wrap) {
  display: none;
}
.tag-switch-row {
  display: flex;
  align-items: center;
  gap: 8px;
}
.tag-select {
  flex: 1;
  min-width: 90px;
}
.switch-label {
  white-space: nowrap;
  font-size: 14px;
  color: #303133;
}
.inline-label {
  font-size: 14px;
  color: #303133;
  white-space: nowrap;
}
.reward-label-first {
  flex: 0 0 var(--label-first-width, 96px);
  text-align: right;
}
.reward-label-second {
  flex: 0 0 var(--label-second-width, 80px);
  margin-left: 20px;
  text-align: right;
}
.reward-label-third {
  flex: 0 0 var(--label-third-width, 40px);
  margin-left: 20px;
  text-align: right;
}
.reward-input-first {
  width: 150px;
}
.reward-input-second {
  width: 90px;
}
.reward-input-third {
  width: 70px;
}
.reward-table {
  border: 1px solid #f0f0f0;
}
.reward-thumb {
  width: 44px;
  height: 44px;
  object-fit: cover;
  border-radius: 4px;
}
.cover-upload-wrap {
  position: relative;
  display: inline-flex;
  flex-direction: column;
  align-items: flex-start;
}
.cover-remove {
  position: absolute;
  top: 4px;
  right: 4px;
  z-index: 2;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  padding: 0;
  font-size: 16px;
  color: #ff4d4f;
  background: transparent;
  border: none;
  cursor: pointer;
  opacity: 0;
  pointer-events: none;
  box-shadow: none;
  outline: none;
  transition: opacity 0.15s ease;
}
.cover-upload-wrap:hover .cover-remove,
.cover-upload-wrap:focus-within .cover-remove {
  opacity: 1;
  pointer-events: auto;
}
.cover-remove:hover {
  color: #cf1322;
}
.cover-remove:focus-visible {
  outline: 1px solid #ff4d4f;
  outline-offset: 1px;
}
.reward-remove {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  padding: 0;
  font-size: 16px;
  color: #ff4d4f;
  background: transparent;
  border: none;
  cursor: pointer;
}
.reward-remove:hover {
  color: #cf1322;
}
.image-upload-box {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  width: 96px;
  height: 96px;
  overflow: hidden;
  color: #8c8c8c;
  border: 1px dashed #d9d9d9;
  border-radius: 6px;
  background: #fafafa;
}
.image-upload-box img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.upload-plus {
  margin-bottom: 4px;
  font-size: 22px;
  line-height: 1;
}
.create-actions {
  display: flex;
  justify-content: center;
  gap: 10px;
  margin: 20px 0 0;
}
.create-actions button {
  min-width: 72px;
}
.btn-create {
  background: #42b983 !important;
  border-color: #42b983 !important;
}
.btn-create:hover {
  background: #36a374 !important;
  border-color: #36a374 !important;
}
@media (max-width: 900px) {
  .form-layout {
    grid-template-columns: 1fr;
  }
}
</style>
