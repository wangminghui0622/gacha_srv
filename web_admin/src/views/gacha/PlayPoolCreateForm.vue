<template>
  <div class="create-form">
    <div class="form-layout">
      <div class="form-left">
      <a-form :label-col="{ span: 5 }" :wrapper-col="{ span: 18 }" label-align="right">
        <a-form-item label="主题名称" required>
          <a-input v-model:value="form.name" placeholder="主题名称" />
        </a-form-item>
        <a-form-item label="IP" required>
          <a-select v-model:value="form.ipId" :options="ipOptions" placeholder="IP" show-search option-filter-prop="label" />
        </a-form-item>
        <a-form-item label="玩法类型" required>
          <a-select v-model:value="form.mode" :options="modeOptions" />
        </a-form-item>

        <a-form-item label="活动开始时间">
          <a-date-picker v-model:value="form.startAt" show-time style="width: 100%" placeholder="活动开始时间" />
        </a-form-item>
        <a-form-item label="活动结束时间">
          <a-date-picker v-model:value="form.endAt" show-time style="width: 100%" placeholder="活动结束时间" />
        </a-form-item>
        <a-form-item label="价格" required>
          <a-input-number v-model:value="form.priceYuan" :min="0" :precision="2" style="width: 100%" />
        </a-form-item>
        <a-form-item label="折扣(%)" required>
          <a-input-number v-model:value="form.discount" :min="0" :max="100" :precision="2" style="width: 100%" />
        </a-form-item>
        <a-form-item label="箱子数量" required>
          <a-input-number v-model:value="form.boxCount" :min="1" :precision="0" style="width: 100%" />
        </a-form-item>
        <a-form-item label="排序权重">
          <a-input-number v-model:value="form.sortWeight" :min="0" :precision="0" style="width: 100%" />
        </a-form-item>

        <a-form-item label="优惠券">
          <div class="coupon-config">
            <a-space>
              <a-input-number v-model:value="couponId" :min="0" :precision="0" placeholder="优惠券id" style="width: 160px" />
              <a-button @click="addCoupon">添加</a-button>
            </a-space>
            <a-table :data-source="coupons" :pagination="false" row-key="id" size="small" :locale="{ emptyText: '暂无数据' }" class="coupon-table">
              <a-table-column title="ID" data-index="id" width="100" />
              <a-table-column title="名称" data-index="name" />
              <a-table-column title="描述" data-index="description" />
              <a-table-column title="状态" data-index="status" width="100" />
              <a-table-column title="操作" width="90">
                <template #default="{ record }">
                  <a-button type="link" danger size="small" @click="removeCoupon(record.id)">删除</a-button>
                </template>
              </a-table-column>
            </a-table>
          </div>
        </a-form-item>

        <a-form-item label="封面图">
          <div class="cover-row">
            <div class="cover-item">
              <div class="cover-label">封面大图</div>
              <a-upload :show-upload-list="false" accept="image/*" :before-upload="(file: File) => uploadImage(file, 'coverImage')">
                <div class="image-upload-box">
                  <img v-if="form.coverImage" :src="form.coverImage" alt="" />
                  <template v-else><PlusOutlined /><span>Upload</span></template>
                </div>
              </a-upload>
            </div>
            <div class="cover-item">
              <div class="cover-label">封面缩略图</div>
              <a-upload :show-upload-list="false" accept="image/*" :before-upload="(file: File) => uploadImage(file, 'coverThumb')">
                <div class="image-upload-box">
                  <img v-if="form.coverThumb" :src="form.coverThumb" alt="" />
                  <template v-else><PlusOutlined /><span>Upload</span></template>
                </div>
              </a-upload>
            </div>
          </div>
        </a-form-item>
      </a-form>
      </div>

      <div class="form-right">
        <div class="prize-section">
          <div class="prize-label">赏品配置：</div>
          <div class="prize-config">
            <a-table :data-source="prizes" :pagination="false" row-key="prizeIndex" size="small" :locale="{ emptyText: '暂无数据' }" :scroll="{ x: 680 }">
              <a-table-column title="序号" data-index="prizeIndex" width="50" align="center" />
              <a-table-column title="赏别" data-index="prizeGrade" width="80" align="center" />
              <a-table-column title="类型" width="70" align="center">
                <template #default="{ record }">{{ record.prizeType === 2 ? '商品' : '虚拟' }}</template>
              </a-table-column>
              <a-table-column title="赏品" data-index="name" align="center" />
              <a-table-column title="缩略图" width="70" align="center">
                <template #default>—</template>
              </a-table-column>
              <a-table-column title="中奖记录" width="80" align="center">
                <template #default="{ record }">{{ record.isPrize ? '是' : '否' }}</template>
              </a-table-column>
              <a-table-column title="成本价" width="80" align="center">
                <template #default="{ record }">{{ Number(record.costFen ?? 0) / 100 }}</template>
              </a-table-column>
              <a-table-column title="总数" data-index="quantity" width="60" align="center" />
              <a-table-column title="概率(%)" width="80" align="center">
                <template #default="{ record }">{{ probability(record) }}</template>
              </a-table-column>
              <a-table-column title="操作" width="80" align="center">
                <template #default="{ record }">
                  <a-button type="link" danger size="small" @click="removePrize(record.prizeIndex)">删除</a-button>
                </template>
              </a-table-column>
            </a-table>

            <div class="prize-inline">
              <a-select v-model:value="prizeDraft.prizeGrade" :options="gradeOptions" style="width: 90px" />
              <a-select v-model:value="prizeDraft.prizeType" :options="prizeTypeOptions" style="width: 85px" />
              <div class="prize-picker">
                <a-button v-if="prizeDraft.prizeType === 2" @click="skuModalOpen = true">选择赏品</a-button>
                <span v-if="prizeDraft.prizeType === 2 && prizeDraft.skuId" class="picked-sku">SKU {{ prizeDraft.skuId }}</span>
                <a-input v-else v-model:value="prizeDraft.name" placeholder="赏品名称" />
              </div>
              <a-switch v-model:checked="prizeDraft.isPrize" />
              <a-input-number v-model:value="prizeDraft.costYuan" :min="0" :precision="2" style="width: 80px" />
              <a-button @click="addPrize">添加</a-button>
              <a-button @click="clearPrizeDraft">清除</a-button>
            </div>
          </div>
        </div>
      </div>
    </div>

      <div class="switch-panel">
        <div class="switch-panel-title">其他设置</div>
        <div class="switch-grid">
          <label v-for="item in switchItems" :key="item.key" class="switch-item">
            <a-switch v-model:checked="form[item.key]" />
            <span>{{ item.label }}</span>
          </label>
        </div>
      </div>

      <div class="create-actions">
        <a-button class="btn-create" type="primary" :loading="saving" @click="submit">创建</a-button>
      </div>

    <a-modal v-model:open="skuModalOpen" title="选择赏品" :confirm-loading="false" width="420px" @ok="confirmSku">
      <a-form layout="vertical">
        <a-form-item label="SKU ID" required>
          <a-input-number v-model:value="skuInput" :min="1" :precision="0" style="width: 100%" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import { PlusOutlined } from '@ant-design/icons-vue'
import type { Dayjs } from 'dayjs'
import { catalogIpClient, type IpItem } from '@/api/catalog'
import { listPrizeGrades, type PrizeGrade } from '@/api/prizeGrade'
import { playPoolClient, type PlayPoolPrize } from '@/api/playPool'

const emit = defineEmits<{ success: [] }>()
const saving = ref(false)
const ips = ref<IpItem[]>([])
const grades = ref<PrizeGrade[]>([])

const modeOptions = [
  { label: '一番赏', value: 'gashapon' },
  { label: '无限赏', value: 'fullcar' },
  { label: '串串赏', value: 'combochain' },
  { label: '挖宝赏', value: 'dig' },
  { label: '拼团赏', value: 'groupbuy' },
  { label: '对对碰', value: 'memorymatch' },
]
const prizeTypeOptions = [
  { label: '商品', value: 2 },
  { label: '虚拟', value: 1 },
]

const form = reactive({
  name: '',
  ipId: undefined as string | undefined,
  mode: 'gashapon',
  startAt: null as Dayjs | null,
  endAt: null as Dayjs | null,
  priceYuan: 1,
  discount: 0,
  isOnSale: false,
  isNew: false,
  isRecommended: false,
  isBargain: false,
  isPresale: false,
  isMiniapp: false,
  isWelfare: false,
  excludeTask: false,
  allowBoxInBox: false,
  boxCount: 1,
  sortWeight: 0,
  coverImage: '',
  coverThumb: '',
})

const switchItems = [
  { key: 'isOnSale', label: '是否上架' },
  { key: 'isNew', label: '是否新品' },
  { key: 'isRecommended', label: '是否推荐' },
  { key: 'isBargain', label: '是否捡漏' },
  { key: 'isPresale', label: '是否预售' },
  { key: 'isMiniapp', label: '是否小程序展示' },
  { key: 'isWelfare', label: '是否福利箱' },
  { key: 'excludeTask', label: '不参与任务' },
  { key: 'allowBoxInBox', label: '允许作为箱中箱' },
] as const

const prizes = ref<PlayPoolPrize[]>([])
const prizeDraft = reactive({
  prizeGrade: 'SP',
  prizeType: 2,
  name: '',
  skuId: 0,
  costYuan: 1,
  isPrize: true,
})
const skuModalOpen = ref(false)
const skuInput = ref<number | null>(null)

const coupons = ref<Array<{ id: number; name: string; description: string; status: string }>>([])
const couponId = ref<number | null>(null)

const ipOptions = computed(() => ips.value.filter((item) => item.ipId && item.name).map((item) => ({ label: item.name as string, value: item.ipId as string })))
const gradeOptions = computed(() => {
  const source = grades.value.length > 0 ? grades.value.map((item) => item.code) : ['SP', 'SSR', 'SS', 'S', 'A', 'B', 'Last']
  return Array.from(new Set(source)).map((code) => ({ label: code, value: code }))
})

async function loadOptions() {
  const [ipReply, gradeReply] = await Promise.all([
    catalogIpClient.List({ keyword: '', page: 1, pageSize: 100 }).catch(() => ({ ips: [] })),
    listPrizeGrades().catch(() => ({ items: [] })),
  ])
  ips.value = ipReply.ips ?? []
  grades.value = gradeReply.items ?? []
}

function addPrize() {
  if (prizeDraft.prizeType === 2 && !prizeDraft.skuId) {
    message.warning('请先选择赏品')
    return
  }
  const name = prizeDraft.prizeType === 2 ? `SKU ${prizeDraft.skuId}` : prizeDraft.name.trim()
  if (!name) {
    message.warning('请输入赏品名称')
    return
  }
  const specialMap: Record<string, string> = { Last: 'LAST', Mvp: 'MVP', Lucky: 'LUCKY', Global: 'GLOBAL' }
  const specialKind = specialMap[prizeDraft.prizeGrade] ?? ''
  const prizeType = specialKind ? 1 : prizeDraft.prizeType
  prizes.value.push({
    prizeIndex: prizes.value.length,
    prizeGrade: prizeDraft.prizeGrade,
    specialKind,
    prizeType,
    name,
    skuId: prizeType === 2 ? prizeDraft.skuId : 0,
    weight: 1,
    quantity: 1,
    remaining: 1,
    isPrize: prizeDraft.isPrize,
    costFen: Math.round((prizeDraft.costYuan || 0) * 100),
    cashFen: specialKind ? 500 : 0,
  })
  clearPrizeDraft()
}

function clearPrizeDraft() {
  prizeDraft.prizeGrade = 'SP'
  prizeDraft.prizeType = 2
  prizeDraft.name = ''
  prizeDraft.skuId = 0
  prizeDraft.costYuan = 1
  prizeDraft.isPrize = true
  skuInput.value = null
}

function removePrize(index: number) {
  prizes.value = prizes.value.filter((item) => item.prizeIndex !== index).map((item, i) => ({ ...item, prizeIndex: i }))
}

function probability(prize: PlayPoolPrize): string {
  const total = prizes.value.reduce((sum, item) => sum + Number(item.weight ?? 0), 0)
  if (total <= 0) return '0'
  return ((Number(prize.weight ?? 0) / total) * 100).toFixed(2)
}

function confirmSku() {
  if (!skuInput.value || skuInput.value <= 0) {
    message.warning('请输入 SKU ID')
    return
  }
  prizeDraft.skuId = skuInput.value
  skuModalOpen.value = false
}

function addCoupon() {
  if (!couponId.value || couponId.value <= 0) return
  if (coupons.value.some((item) => item.id === couponId.value)) return
  coupons.value.push({ id: couponId.value, name: '-', description: '-', status: '待查询' })
  couponId.value = null
}

function removeCoupon(id: number) {
  coupons.value = coupons.value.filter((item) => item.id !== id)
}

function readImage(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result ?? ''))
    reader.onerror = () => reject(reader.error)
    reader.readAsDataURL(file)
  })
}

async function uploadImage(file: File, field: 'coverImage' | 'coverThumb'): Promise<boolean> {
  if (!file.type.startsWith('image/')) {
    message.warning('请选择图片文件')
    return false
  }
  if (file.size > 2 * 1024 * 1024) {
    message.warning('图片不能超过 2MB')
    return false
  }
  try {
    form[field] = await readImage(file)
  } catch {
    // 忽略读取失败
  }
  return false
}

async function submit() {
  if (!form.name.trim()) {
    message.warning('请输入主题名称')
    return
  }
  if (!form.ipId) {
    message.warning('请选择 IP')
    return
  }
  if (prizes.value.length === 0) {
    message.warning('请至少添加一个赏品')
    return
  }
  const tags: string[] = []
  if (form.isOnSale) tags.push('展示')
  if (form.isNew) tags.push('新品')
  if (form.isRecommended) tags.push('推荐')
  if (form.isBargain) tags.push('捡漏')
  if (form.isPresale) tags.push('预售')
  const adminJson = JSON.stringify({
    startAt: form.startAt ? form.startAt.format('YYYY-MM-DD HH:mm:ss') : '',
    endAt: form.endAt ? form.endAt.format('YYYY-MM-DD HH:mm:ss') : '',
    discount: form.discount,
    isOnSale: form.isOnSale,
    isNew: form.isNew,
    isRecommended: form.isRecommended,
    isBargain: form.isBargain,
    isPresale: form.isPresale,
    isMiniapp: form.isMiniapp,
    isWelfare: form.isWelfare,
    excludeTask: form.excludeTask,
    allowBoxInBox: form.allowBoxInBox,
    coverImage: form.coverImage,
    coverThumb: form.coverThumb,
    coupons: coupons.value,
  })
  const configs: Record<string, string> = {
    gashapon: '{}',
    fullcar: '{"seatCount":4}',
    combochain: '{"ticketsPerCar":2,"totalCars":10,"tierTriggerProbabilityBps":300}',
    dig: '{"cellCount":9}',
    groupbuy: '{"targetBetCount":3,"perBetAwardLimit":0}',
    memorymatch: '{"roundsPerPurchase":3,"maxPairCount":3,"patternWeights":[1,1,1,1,1,1,1,1,1],"facePresets":[{"presetId":1,"name":"默认","faces":[]}],"backPresets":[{"presetId":1,"name":"默认","backImageUrl":"","faceBackgroundImageUrl":""}]}',
  }
  saving.value = true
  try {
    await playPoolClient.CreateDraft({
      idempotencyKey: crypto.randomUUID(),
      mode: form.mode,
      name: form.name.trim(),
      priceFen: Math.round((form.priceYuan || 0) * 100),
      configJson: configs[form.mode] ?? '{}',
      prizes: prizes.value,
      ipId: form.ipId,
      tags,
      periods: form.boxCount,
      weight: form.sortWeight,
      adminJson,
    })
    message.success('赏池已创建')
    emit('success')
  } catch {
    message.error('创建失败，请检查赏品配置')
  } finally {
    saving.value = false
  }
}

onMounted(loadOptions)
</script>

<style scoped lang="scss">
.create-form {
  width: 100%;
}

.form-layout {
  display: grid;
  grid-template-columns: minmax(360px, 36%) minmax(0, 1fr);
  gap: 28px;
  align-items: start;
}

.form-left,
.form-right {
  min-width: 0;
}

.prize-section {
  margin-bottom: 0;
}

.prize-label {
  padding: 6px 0 10px;
  color: #606266;
  font-size: 14px;
  font-weight: 500;
  text-align: left;
}

.prize-config {
  width: 100%;
  min-width: 0;
}

.prize-inline {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-top: 8px;
  padding: 8px;
  border-top: 1px solid #f0f0f0;
}

.prize-picker {
  display: flex;
  flex: 1;
  align-items: center;
  gap: 8px;
}

.picked-sku {
  color: #606266;
}

.inline-label {
  color: #606266;
  font-size: 13px;
  white-space: nowrap;
}

.coupon-config {
  width: 100%;
}

.coupon-table {
  margin-top: 10px;
}

.cover-row {
  display: flex;
  align-items: flex-start;
  gap: 40px;
}

.cover-item {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.cover-label {
  color: #606266;
  font-size: 13px;
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

.image-upload-box span {
  margin-top: 4px;
  font-size: 12px;
}

.switch-panel {
  margin: 8px 0 0 20.8%;
  padding: 16px 20px;
  background: #fafafa;
  border-top: 1px solid #f0f0f0;
  border-radius: 4px;
}

.switch-panel-title {
  margin-bottom: 12px;
  font-size: 14px;
  font-weight: 600;
  color: #303133;
}

.switch-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(160px, 1fr));
  gap: 12px 28px;
}

.switch-item {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #606266;
  cursor: pointer;
}

.create-actions {
  margin: 16px 0 0 20.8%;
}

.btn-create {
  min-width: 72px;
  background: #42b983 !important;
  border-color: #42b983 !important;
}

.btn-create:hover {
  background: #36a374 !important;
  border-color: #36a374 !important;
}
</style>
