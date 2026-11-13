<template>
  <div>
    <PageHeader :title="$t('nav.gacha')" :show-back="false">
      <template #extra>
        <a-space>
          <a-input v-model:value="modeFilter" placeholder="按玩法筛选（可选）" allow-clear style="width: 180px" @press-enter="load" />
          <a-button :loading="loading" @click="load">刷新</a-button>
          <a-button v-if="canCreate" type="primary" @click="openCreate">新建赏池</a-button>
        </a-space>
      </template>
    </PageHeader>

    <a-table :data-source="pools" :loading="loading" :pagination="false" :row-key="(row: PlayPoolSummary) => String(row.poolId)" size="middle" :scroll="{ x: 820 }">
      <a-table-column title="赏池 ID" data-index="poolId" width="110" />
      <a-table-column title="玩法" width="130">
        <template #default="{ record }">{{ modeText(record.mode) }}</template>
      </a-table-column>
      <a-table-column title="名称" data-index="name" />
      <a-table-column title="最新版本" data-index="latestVersion" width="100" />
      <a-table-column title="状态" width="110">
        <template #default="{ record }"><a-tag :color="statusColor(record.status)">{{ statusText(record.status) }}</a-tag></template>
      </a-table-column>
      <a-table-column title="发布时间" width="180">
        <template #default="{ record }">{{ timeText(record.publishedAt) }}</template>
      </a-table-column>
      <a-table-column title="操作" width="170">
        <template #default="{ record }">
          <a-space>
            <a @click="openDetail(record)">查看</a>
            <a v-if="canPublish && isDraft(record.status)" @click="publish(record)">发布</a>
          </a-space>
        </template>
      </a-table-column>
    </a-table>

    <a-space class="pager">
      <a-button :disabled="loading || !afterPoolId" @click="previous">上一页</a-button>
      <a-button :disabled="loading || !hasMore" @click="next">下一页</a-button>
    </a-space>

    <a-modal v-model:open="createOpen" title="新建赏池草稿" :confirm-loading="creating" width="760px" @ok="create">
      <a-form layout="vertical">
        <a-row :gutter="16">
          <a-col :xs="24" :md="8">
            <a-form-item label="玩法" required>
              <a-select v-model:value="form.mode" :options="modeOptions" @change="resetSkeleton" />
            </a-form-item>
          </a-col>
          <a-col :xs="24" :md="10">
            <a-form-item label="名称" required><a-input v-model:value="form.name" /></a-form-item>
          </a-col>
          <a-col :xs="24" :md="6">
            <a-form-item label="单价（分）"><a-input-number v-model:value="form.priceFen" :min="0" style="width: 100%" /></a-form-item>
          </a-col>
        </a-row>
        <a-form-item label="玩法配置 configJson">
          <a-textarea v-model:value="form.configJson" :rows="5" spellcheck="false" />
        </a-form-item>
        <a-form-item label="奖品 prizes（JSON 数组）">
          <a-textarea v-model:value="form.prizesJson" :rows="12" spellcheck="false" />
        </a-form-item>
      </a-form>
    </a-modal>

    <a-drawer v-model:open="detailOpen" title="赏池版本" width="640">
      <a-spin :spinning="detailLoading">
        <template v-if="detail">
          <a-descriptions bordered :column="2" size="small">
            <a-descriptions-item label="赏池 ID">{{ detail.poolId }}</a-descriptions-item>
            <a-descriptions-item label="版本">{{ detail.version }}</a-descriptions-item>
            <a-descriptions-item label="玩法">{{ modeText(detail.mode) }}</a-descriptions-item>
            <a-descriptions-item label="状态">{{ statusText(detail.status) }}</a-descriptions-item>
            <a-descriptions-item label="名称" :span="2">{{ detail.name }}</a-descriptions-item>
            <a-descriptions-item label="单价（分）">{{ detail.priceFen }}</a-descriptions-item>
            <a-descriptions-item label="配置哈希">{{ detail.configHash }}</a-descriptions-item>
          </a-descriptions>
          <h4>configJson</h4>
          <pre class="json-block">{{ detail.configJson }}</pre>
          <h4>奖品</h4>
          <a-table :data-source="detail.prizes ?? []" :pagination="false" row-key="prizeIndex" size="small">
            <a-table-column title="#" data-index="prizeIndex" width="50" />
            <a-table-column title="名称" data-index="name" />
            <a-table-column title="类别" width="110">
              <template #default="{ record }">{{ record.specialKind || '普通' }}</template>
            </a-table-column>
            <a-table-column title="类型" width="70">
              <template #default="{ record }">{{ record.prizeType === 2 ? '实物' : record.prizeType === 3 ? '箱子' : '虚拟' }}</template>
            </a-table-column>
            <a-table-column title="SKU" data-index="skuId" width="80" />
            <a-table-column title="权重" data-index="weight" width="70" />
            <a-table-column title="剩余/总量" width="110">
              <template #default="{ record }">{{ record.remaining }} / {{ record.quantity }}</template>
            </a-table-column>
          </a-table>
        </template>
      </a-spin>
    </a-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import PageHeader from '@/components/PageHeader.vue'
import { allows } from '@/access/grants'
import { useSessionStore } from '@/stores/session'
import { playPoolClient, type PlayPoolPrize, type PlayPoolSummary, type PlayPoolVersion } from '@/api/playPool'

const session = useSessionStore()
const canCreate = computed(() => allows(session.permissionNodes, 'gacha.pool', 'create'))
const canPublish = computed(() => allows(session.permissionNodes, 'gacha.pool', 'publish'))

const pools = ref<PlayPoolSummary[]>([])
const loading = ref(false)
const afterPoolId = ref('')
const history = ref<string[]>([])
const hasMore = ref(false)
const modeFilter = ref('')

const createOpen = ref(false)
const creating = ref(false)
const form = reactive({ mode: 'gashapon', name: '', priceFen: 100, configJson: '', prizesJson: '' })

const detailOpen = ref(false)
const detailLoading = ref(false)
const detail = ref<PlayPoolVersion | null>(null)

const modeOptions = [
  { label: '扭蛋 gashapon', value: 'gashapon' },
  { label: '整车 fullcar', value: 'fullcar' },
  { label: '连击链 combochain', value: 'combochain' },
  { label: '挖宝箱 dig', value: 'dig' },
  { label: '拼团 groupbuy', value: 'groupbuy' },
  { label: '记忆翻牌 memorymatch', value: 'memorymatch' },
]

const modeNames: Record<string, string> = {
  gashapon: '扭蛋', fullcar: '整车', combochain: '连击链', dig: '挖宝箱', groupbuy: '拼团', memorymatch: '记忆翻牌',
}

const configSkeleton: Record<string, string> = {
  gashapon: '{}',
  fullcar: '{"seatCount":4}',
  combochain: '{"ticketsPerCar":2,"totalCars":10,"tierTriggerProbabilityBps":300}',
  dig: '{"cellCount":9}',
  groupbuy: '{"targetBetCount":3,"perBetAwardLimit":0}',
  memorymatch: '{"roundsPerPurchase":3,"maxPairCount":3,"patternWeights":[1,1,1,1,1,1,1,1,1],"facePresets":[{"presetId":1,"name":"默认","faces":[]}],"backPresets":[{"presetId":1,"name":"默认","backImageUrl":"","faceBackgroundImageUrl":""}]}',
}

const prizeSkeleton: Record<string, unknown[]> = {
  gashapon: [
    { prizeIndex: 0, prizeType: 1, name: '普通赏', weight: 10, quantity: 10, isPrize: true },
    { prizeIndex: 1, prizeType: 1, name: 'LAST赏', specialKind: 'LAST', cashFen: 500, quantity: 1, isPrize: true },
  ],
  fullcar: [
    { prizeIndex: 0, prizeType: 1, name: '谢谢参与', weight: 30, quantity: -1, isPrize: false },
    { prizeIndex: 1, prizeType: 2, name: '实物手办', skuId: 1, weight: 10, quantity: 5, isPrize: true },
    { prizeIndex: 2, prizeType: 1, name: '整车结算', specialKind: 'SETTLEMENT', targetValue: 1, weight: 1, cashFen: 500, quantity: 0 },
  ],
  combochain: [
    { prizeIndex: 0, prizeType: 2, name: '实物手办', skuId: 1, specialKind: 'GLOBAL', targetValue: 0, weight: 1, quantity: 1, isPrize: true },
    { prizeIndex: 1, prizeType: 1, name: '连击奖', specialKind: 'COMBO_TIER', weight: 1, quantity: 1, cashFen: 100, isPrize: true, extraJson: '{"tierWeight":1}' },
  ],
  dig: [
    { prizeIndex: 0, prizeType: 1, name: '现金格', targetValue: 0, cashFen: 50, isPrize: true },
    { prizeIndex: 1, prizeType: 1, name: '空格', targetValue: 1, isPrize: false },
  ],
  groupbuy: [
    { prizeIndex: 0, prizeType: 1, name: '普通奖', weight: 10, quantity: 2, isPrize: true },
    { prizeIndex: 1, prizeType: 1, name: '幸运奖', specialKind: 'LUCKY', cashFen: 500, quantity: 1, isPrize: true },
  ],
  memorymatch: [
    { prizeIndex: 0, prizeType: 1, name: '现金档', targetValue: 3, weight: 1, cashFen: 500, quantity: 10, isPrize: true },
    { prizeIndex: 1, prizeType: 1, name: '谢谢参与', targetValue: 0, weight: 1, quantity: -1, isPrize: false },
  ],
}

function resetSkeleton() {
  form.configJson = configSkeleton[form.mode] ?? '{}'
  form.prizesJson = JSON.stringify(prizeSkeleton[form.mode] ?? [], null, 2)
}

function modeText(mode?: string): string {
  return mode ? modeNames[mode] ?? mode : '—'
}

function statusText(status?: string | number): string {
  const value = String(status ?? '')
  if (value === '2') return '已发布'
  if (value === '3') return '已下架'
  return '草稿'
}

function statusColor(status?: string | number): string {
  const value = String(status ?? '')
  if (value === '2') return 'green'
  if (value === '3') return 'default'
  return 'blue'
}

function isDraft(status?: string | number): boolean {
  return statusText(status) === '草稿'
}

function timeText(value?: string): string {
  if (!value || value === '0') return '—'
  return new Date(Number(value) * 1000).toLocaleString()
}

async function load() {
  loading.value = true
  try {
    const result = await playPoolClient.List({ afterPoolId: afterPoolId.value || undefined, pageSize: 20, mode: modeFilter.value || undefined })
    pools.value = result.items ?? []
    hasMore.value = pools.value.length >= 20
  } catch {
    pools.value = []
    hasMore.value = false
    message.error('赏池列表加载失败')
  } finally {
    loading.value = false
  }
}

function next() {
  const last = pools.value.at(-1)?.poolId
  if (!last) return
  history.value.push(afterPoolId.value)
  afterPoolId.value = String(last)
  void load()
}

function previous() {
  afterPoolId.value = history.value.pop() ?? ''
  void load()
}

function openCreate() {
  form.mode = 'gashapon'
  form.name = ''
  form.priceFen = 100
  resetSkeleton()
  createOpen.value = true
}

async function create() {
  if (!form.name.trim()) {
    message.warning('请填写赏池名称')
    return
  }
  let prizes: unknown[]
  try {
    prizes = JSON.parse(form.prizesJson)
    if (!Array.isArray(prizes) || prizes.length === 0) throw new Error('empty')
  } catch {
    message.error('奖品 JSON 必须是数组且不能为空')
    return
  }
  if (!form.configJson.trim()) form.configJson = '{}'
  try {
    JSON.parse(form.configJson)
  } catch {
    message.error('configJson 不是合法 JSON')
    return
  }
  creating.value = true
  try {
    await playPoolClient.CreateDraft({
      idempotencyKey: crypto.randomUUID(),
      mode: form.mode,
      name: form.name.trim(),
      priceFen: form.priceFen,
      configJson: form.configJson,
      prizes: prizes as PlayPoolPrize[],
    })
    message.success('赏池草稿已创建')
    createOpen.value = false
    await load()
  } catch {
    message.error('创建失败，请检查配置与奖品')
  } finally {
    creating.value = false
  }
}

async function publish(record: PlayPoolSummary) {
  try {
    await playPoolClient.Publish({ poolId: String(record.poolId), version: String(record.latestVersion) })
    message.success('赏池已发布')
    await load()
  } catch {
    message.error('发布失败，请检查奖品/SKU 配置')
  }
}

async function openDetail(record: PlayPoolSummary) {
  detailOpen.value = true
  detailLoading.value = true
  detail.value = null
  try {
    detail.value = await playPoolClient.Get({ poolId: String(record.poolId), version: String(record.latestVersion) })
  } catch {
    message.error('赏池详情加载失败')
  } finally {
    detailLoading.value = false
  }
}

onMounted(load)
</script>

<style scoped lang="scss">
.pager {
  margin-top: 16px;
}

.json-block {
  background: #f6f8fa;
  border-radius: 6px;
  padding: 12px;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 220px;
  overflow: auto;
}

h4 {
  margin: 18px 0 8px;
}
</style>
