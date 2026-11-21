<template>
  <div class="gacha-match-page">
    <PageHeader :title="$t('nav.gachaMatch')" :show-back="false" />
    <p class="page-sub">{{ $t('gachaMatch.subtitle') }}</p>

    <a-tabs v-model:activeKey="activeTab" class="match-tabs">
      <a-tab-pane key="rules" :tab="$t('gachaMatch.tabs.rules')">
        <div class="rules-panel">
          <a-alert type="info" show-icon :message="$t('gachaMatch.rulesNotice')" class="rules-alert" />
          <a-form :label-col="{ span: 9 }" :wrapper-col="{ span: 9 }" class="rules-form">
            <a-form-item :label="$t('gachaMatch.pairProbabilityMode')">
              <a-select v-model:value="rulesForm.pairProbabilityMode" :options="probabilityOptions" />
            </a-form-item>
            <a-form-item :label="$t('gachaMatch.initialHandCount')">
              <a-input-number v-model:value="rulesForm.initialHandCount" :min="1" :max="9" class="number-input" />
            </a-form-item>
            <a-form-item :label="$t('gachaMatch.pairSize')">
              <a-input-number v-model:value="rulesForm.pairSize" :min="2" :max="6" class="number-input" />
            </a-form-item>
            <a-form-item :label="$t('gachaMatch.replenishCount')">
              <a-input-number v-model:value="rulesForm.replenishCount" :min="0" :max="54" class="number-input" />
            </a-form-item>
            <a-form-item :label="$t('gachaMatch.secondDestiny')">
              <a-switch v-model:checked="rulesForm.secondDestiny" />
            </a-form-item>
            <a-form-item :label="$t('gachaMatch.destinyDouble')">
              <a-switch v-model:checked="rulesForm.destinyDouble" />
            </a-form-item>
            <a-form-item :label="$t('gachaMatch.destinyIncome')">
              <a-input-number v-model:value="rulesForm.destinyIncome" :min="0" class="number-input" />
            </a-form-item>
            <a-form-item :label="$t('gachaMatch.doubleDestinyBuff')">
              <a-input-number v-model:value="rulesForm.doubleDestinyBuff" :min="0" class="number-input" />
            </a-form-item>
            <a-form-item :label="$t('gachaMatch.destinyReplenishCost')">
              <a-input-number v-model:value="rulesForm.destinyReplenishCost" :min="0" class="number-input" />
            </a-form-item>
            <a-form-item :label="$t('gachaMatch.resume')">
              <a-switch v-model:checked="rulesForm.resume" />
            </a-form-item>
            <a-form-item :label="$t('gachaMatch.hostMode')">
              <a-switch v-model:checked="rulesForm.hostMode" />
            </a-form-item>
            <a-form-item :label="$t('gachaMatch.hostMinPairs')">
              <a-input-number v-model:value="rulesForm.hostMinPairs" :min="0" class="number-input" />
            </a-form-item>
            <a-form-item :label="$t('gachaMatch.hostShareBps')">
              <a-input-number v-model:value="rulesForm.hostShareBps" :min="0" :max="10000" class="number-input" />
            </a-form-item>
            <a-form-item :label="$t('gachaMatch.revenueBase')">
              <a-select v-model:value="rulesForm.revenueBase" :options="revenueOptions" />
            </a-form-item>
            <a-form-item :label="$t('gachaMatch.ledgerType')">
              <a-input-number v-model:value="rulesForm.ledgerType" class="number-input" />
            </a-form-item>
          </a-form>
          <div class="rules-actions">
            <a-button type="primary" :loading="rulesSaving" @click="saveRules">
              {{ $t('gachaMatch.saveRules') }}
            </a-button>
          </div>
        </div>
      </a-tab-pane>

      <a-tab-pane key="deck" :tab="$t('gachaMatch.tabs.deck')">
        <div class="toolbar">
          <a-button type="primary" @click="openDeckModal()">{{ $t('gachaMatch.addDeck') }}</a-button>
          <a-button @click="loadDecks">{{ $t('common.refresh') }}</a-button>
        </div>
        <a-table
          :data-source="decks"
          :loading="deckLoading"
          :pagination="deckPagination"
          row-key="id"
          size="middle"
          :scroll="{ x: 860 }"
        >
          <a-table-column :title="$t('common.id')" data-index="id" width="90" align="center" />
          <a-table-column :title="$t('gachaMatch.deckName')" data-index="name" align="center" />
          <a-table-column :title="$t('common.status')" width="110" align="center">
            <template #default="{ record }">
              <a-tag :color="Number(record.status) === 1 ? 'green' : 'default'">
                {{ Number(record.status) === 1 ? $t('gachaMatch.enabled') : $t('gachaMatch.disabled') }}
              </a-tag>
            </template>
          </a-table-column>
          <a-table-column :title="$t('gachaMatch.deckTotalCards')" data-index="totalCards" width="120" align="center" />
          <a-table-column :title="$t('poolAdmin.weight')" data-index="weight" width="100" align="center" />
          <a-table-column :title="$t('common.action')" width="180" align="center">
            <template #default="{ record }">
              <a-button type="link" @click="openDeckModal(record)">{{ $t('gachaMatch.edit') }}</a-button>
              <a-popconfirm
                :title="$t('gachaMatch.deckDeleteConfirm')"
                :ok-text="$t('gachaMatch.confirm')"
                :cancel-text="$t('gachaMatch.cancel')"
                @confirm="deleteDeck(record)"
              >
                <a-button type="link" danger>{{ $t('gachaMatch.delete') }}</a-button>
              </a-popconfirm>
            </template>
          </a-table-column>
        </a-table>
      </a-tab-pane>

      <a-tab-pane key="appearance" :tab="$t('gachaMatch.tabs.appearance')">
        <a-alert type="info" show-icon :message="$t('gachaMatch.appearanceNotice')" class="rules-alert" />
        <div class="toolbar">
          <a-select v-model:value="appearanceQuery.type" :options="appearanceTypeOptions" class="filter-select" />
          <a-select v-model:value="appearanceQuery.status" :options="appearanceStatusOptions" class="filter-select" />
          <a-input
            v-model:value="appearanceQuery.keyword"
            :placeholder="$t('gachaMatch.deckName')"
            allow-clear
            class="filter-input"
          />
          <a-button type="primary" @click="loadAppearances(1)">{{ $t('common.query') }}</a-button>
          <a-button @click="openAppearanceModal()">{{ $t('gachaMatch.addAppearance') }}</a-button>
        </div>
        <a-table
          :data-source="appearances"
          :loading="appearanceLoading"
          :pagination="appearancePagination"
          row-key="id"
          size="middle"
          :scroll="{ x: 1500 }"
        >
          <a-table-column :title="$t('common.id')" data-index="id" width="80" align="center" />
          <a-table-column :title="$t('gachaMatch.appearanceType')" width="100" align="center">
            <template #default="{ record }">
              {{ Number(record.type) === 2 ? $t('gachaMatch.cardBack') : $t('gachaMatch.cardFace') }}
            </template>
          </a-table-column>
          <a-table-column :title="$t('gachaMatch.deckName')" data-index="name" width="140" align="center" />
          <a-table-column :title="$t('gachaMatch.appearanceImage')" width="120" align="center">
            <template #default="{ record }">
              <a-image v-if="record.imageUrl" :src="record.imageUrl" :width="48" :height="48" />
              <span v-else>-</span>
            </template>
          </a-table-column>
          <a-table-column :title="$t('gachaMatch.appearanceBackImage')" width="120" align="center">
            <template #default="{ record }">
              <a-image v-if="record.backImageUrl" :src="record.backImageUrl" :width="48" :height="48" />
              <span v-else>-</span>
            </template>
          </a-table-column>
          <a-table-column :title="$t('common.status')" width="100" align="center">
            <template #default="{ record }">
              <a-tag :color="Number(record.status) === 1 ? 'green' : 'default'">
                {{ Number(record.status) === 1 ? $t('gachaMatch.enabled') : $t('gachaMatch.disabled') }}
              </a-tag>
            </template>
          </a-table-column>
          <a-table-column :title="$t('gachaMatch.appearanceFree')" width="90" align="center">
            <template #default="{ record }">
              {{ Number(record.free) === 1 ? $t('gachaMatch.freeYes') : $t('gachaMatch.freeNo') }}
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('gachaMatch.appearanceExchangePoints')"
            data-index="exchangePoints"
            width="120"
            align="center"
          />
          <a-table-column
            :title="$t('gachaMatch.appearanceFunctionType')"
            data-index="functionType"
            width="140"
            align="center"
          />
          <a-table-column
            :title="$t('gachaMatch.appearanceFunctionConfig')"
            data-index="functionConfig"
            width="180"
            align="center"
          />
          <a-table-column :title="$t('poolAdmin.weight')" data-index="weight" width="90" align="center" />
          <a-table-column :title="$t('common.action')" width="170" align="center">
            <template #default="{ record }">
              <a-button type="link" @click="openAppearanceModal(record)">{{ $t('gachaMatch.edit') }}</a-button>
              <a-popconfirm
                :title="$t('gachaMatch.appearanceDeleteConfirm')"
                :ok-text="$t('gachaMatch.confirm')"
                :cancel-text="$t('gachaMatch.cancel')"
                @confirm="deleteAppearance(record)"
              >
                <a-button type="link" danger>{{ $t('gachaMatch.delete') }}</a-button>
              </a-popconfirm>
            </template>
          </a-table-column>
        </a-table>
      </a-tab-pane>
    </a-tabs>

    <a-modal
      v-model:open="deckModalOpen"
      :title="deckForm.id ? $t('gachaMatch.deckEditTitle') : $t('gachaMatch.deckCreateTitle')"
      :confirm-loading="deckSaving"
      @ok="saveDeck"
    >
      <a-form :label-col="{ span: 7 }" :wrapper-col="{ span: 15 }">
        <a-form-item :label="$t('gachaMatch.deckName')">
          <a-input v-model:value="deckForm.name" :placeholder="$t('gachaMatch.nameRequired')" />
        </a-form-item>
        <a-form-item :label="$t('common.status')">
          <a-select v-model:value="deckForm.status" :options="statusOptions" />
        </a-form-item>
        <a-form-item :label="$t('gachaMatch.deckTotalCards')">
          <a-input-number v-model:value="deckForm.totalCards" :min="1" :max="54" class="number-input" />
        </a-form-item>
        <a-form-item :label="$t('poolAdmin.weight')">
          <a-input-number v-model:value="deckForm.weight" :min="0" class="number-input" />
        </a-form-item>
        <a-form-item :label="$t('gachaMatch.deckConfig')">
          <a-textarea
            v-model:value="deckForm.configJson"
            :rows="4"
            :placeholder="$t('gachaMatch.deckConfigPlaceholder')"
          />
        </a-form-item>
      </a-form>
    </a-modal>

    <a-modal
      v-model:open="appearanceModalOpen"
      :title="appearanceForm.id ? $t('gachaMatch.appearanceEditTitle') : $t('gachaMatch.appearanceCreateTitle')"
      :confirm-loading="appearanceSaving"
      width="640px"
      @ok="saveAppearance"
    >
      <a-form :label-col="{ span: 7 }" :wrapper-col="{ span: 16 }">
        <a-form-item :label="$t('gachaMatch.appearanceType')">
          <a-select v-model:value="appearanceForm.type" :options="appearanceTypeEditOptions" />
        </a-form-item>
        <a-form-item :label="$t('gachaMatch.deckName')">
          <a-input v-model:value="appearanceForm.name" :placeholder="$t('gachaMatch.nameRequired')" />
        </a-form-item>
        <a-form-item :label="$t('gachaMatch.imageUrl')">
          <div class="upload-row">
            <a-input v-model:value="appearanceForm.imageUrl" />
            <a-upload
              :show-upload-list="false"
              accept="image/*"
              :before-upload="(file: File) => uploadAppearanceImage(file, 'imageUrl')"
            >
              <a-button :loading="uploadingField === 'imageUrl'">{{ $t('gachaMatch.upload') }}</a-button>
            </a-upload>
          </div>
          <a-image v-if="appearanceForm.imageUrl" :src="appearanceForm.imageUrl" :width="72" :height="72" class="upload-preview" />
        </a-form-item>
        <a-form-item :label="$t('gachaMatch.backImageUrl')">
          <div class="upload-row">
            <a-input v-model:value="appearanceForm.backImageUrl" />
            <a-upload
              :show-upload-list="false"
              accept="image/*"
              :before-upload="(file: File) => uploadAppearanceImage(file, 'backImageUrl')"
            >
              <a-button :loading="uploadingField === 'backImageUrl'">{{ $t('gachaMatch.upload') }}</a-button>
            </a-upload>
          </div>
          <a-image
            v-if="appearanceForm.backImageUrl"
            :src="appearanceForm.backImageUrl"
            :width="72"
            :height="72"
            class="upload-preview"
          />
        </a-form-item>
        <a-form-item :label="$t('common.status')">
          <a-select v-model:value="appearanceForm.status" :options="statusOptions" />
        </a-form-item>
        <a-form-item :label="$t('gachaMatch.appearanceFree')">
          <a-switch v-model:checked="appearanceForm.free" />
        </a-form-item>
        <a-form-item :label="$t('gachaMatch.appearanceExchangePoints')">
          <a-input-number v-model:value="appearanceForm.exchangePoints" :min="0" :precision="2" class="number-input" />
        </a-form-item>
        <a-form-item :label="$t('gachaMatch.appearanceFunctionType')">
          <a-input v-model:value="appearanceForm.functionType" />
        </a-form-item>
        <a-form-item :label="$t('gachaMatch.appearanceFunctionConfig')">
          <a-input v-model:value="appearanceForm.functionConfig" />
        </a-form-item>
        <a-form-item :label="$t('poolAdmin.weight')">
          <a-input-number v-model:value="appearanceForm.weight" :min="0" class="number-input" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import PageHeader from '@/components/PageHeader.vue'
import {
  gachaMatchClient,
  type MemoryMatchAppearance,
  type MemoryMatchDeckTemplate,
} from '@/api/gachaMatch'
import { uploadFileToOss } from '@/api/upload'

const { t } = useI18n()
const activeTab = ref('rules')

const probabilityOptions = computed(() => [
  { value: 0, label: t('gachaMatch.naturalMode') },
  { value: 1, label: t('gachaMatch.exactAllocationMode') },
])
const revenueOptions = computed(() => [
  { value: 1, label: t('gachaMatch.revenueActual') },
  { value: 2, label: t('gachaMatch.revenueOriginal') },
])
const statusOptions = computed(() => [
  { value: 1, label: t('gachaMatch.enabled') },
  { value: 0, label: t('gachaMatch.disabled') },
])

const rulesForm = reactive({
  name: '',
  pairProbabilityMode: 0,
  initialHandCount: 9,
  pairSize: 2,
  replenishCount: 1,
  secondDestiny: true,
  destinyDouble: true,
  destinyIncome: 1,
  doubleDestinyBuff: 1,
  destinyReplenishCost: 1,
  resume: true,
  hostMode: true,
  hostMinPairs: 6,
  hostShareBps: 50,
  revenueBase: 1,
  ledgerType: 0,
  status: 1,
})
const rulesSaving = ref(false)

function toNumber(value: unknown, fallback = 0): number {
  const n = Number(value)
  return Number.isFinite(n) ? n : fallback
}

async function loadRules() {
  try {
    const reply = await gachaMatchClient.GetRules()
    const rules = reply.rules
    if (!rules) return
    rulesForm.name = rules.name ?? ''
    rulesForm.pairProbabilityMode = toNumber(rules.pairProbabilityMode)
    rulesForm.initialHandCount = toNumber(rules.initialHandCount, 9)
    rulesForm.pairSize = toNumber(rules.pairSize, 2)
    rulesForm.replenishCount = toNumber(rules.replenishCount, 1)
    rulesForm.secondDestiny = toNumber(rules.secondDestinyEnabled, 1) === 1
    rulesForm.destinyDouble = toNumber(rules.destinyDoubleEnabled, 1) === 1
    rulesForm.destinyIncome = toNumber(rules.destinyIncome, 1)
    rulesForm.doubleDestinyBuff = toNumber(rules.doubleDestinyBuff, 1)
    rulesForm.destinyReplenishCost = toNumber(rules.destinyReplenishCost, 1)
    rulesForm.resume = toNumber(rules.resumeEnabled, 1) === 1
    rulesForm.hostMode = toNumber(rules.hostModeEnabled, 1) === 1
    rulesForm.hostMinPairs = toNumber(rules.hostMinPairs, 6)
    rulesForm.hostShareBps = toNumber(rules.hostShareBps, 50)
    rulesForm.revenueBase = toNumber(rules.revenueBase, 1)
    rulesForm.ledgerType = toNumber(rules.ledgerType)
    rulesForm.status = toNumber(rules.status, 1)
  } catch {
    message.error(t('gachaMatch.rulesLoadFailed'))
  }
}

async function saveRules() {
  rulesSaving.value = true
  try {
    await gachaMatchClient.SaveRules({
      id: 1,
      name: rulesForm.name,
      pairProbabilityMode: rulesForm.pairProbabilityMode,
      initialHandCount: rulesForm.initialHandCount,
      pairSize: rulesForm.pairSize,
      replenishCount: rulesForm.replenishCount,
      secondDestinyEnabled: rulesForm.secondDestiny ? 1 : 0,
      destinyDoubleEnabled: rulesForm.destinyDouble ? 1 : 0,
      destinyIncome: rulesForm.destinyIncome,
      doubleDestinyBuff: rulesForm.doubleDestinyBuff,
      destinyReplenishCost: rulesForm.destinyReplenishCost,
      resumeEnabled: rulesForm.resume ? 1 : 0,
      hostModeEnabled: rulesForm.hostMode ? 1 : 0,
      hostMinPairs: rulesForm.hostMinPairs,
      hostShareBps: rulesForm.hostShareBps,
      revenueBase: rulesForm.revenueBase,
      ledgerType: rulesForm.ledgerType,
      status: rulesForm.status,
    })
    message.success(t('gachaMatch.rulesSaved'))
  } catch {
    message.error(t('gachaMatch.rulesSaveFailed'))
  } finally {
    rulesSaving.value = false
  }
}

const decks = ref<MemoryMatchDeckTemplate[]>([])
const deckLoading = ref(false)
const deckPage = ref(1)
const deckPageSize = ref(20)
const deckTotal = ref(0)
const deckModalOpen = ref(false)
const deckSaving = ref(false)
const deckForm = reactive({
  id: 0,
  name: '',
  status: 1,
  totalCards: 54,
  weight: 0,
  configJson: '{}',
})

const deckPagination = computed(() => ({
  current: deckPage.value,
  pageSize: deckPageSize.value,
  total: deckTotal.value,
  showSizeChanger: true,
  showTotal: (total: number) => t('gachaMatch.totalItems', { total }),
  onChange: (page: number, pageSize: number) => {
    deckPage.value = page
    deckPageSize.value = pageSize
    void loadDecks()
  },
}))

async function loadDecks() {
  deckLoading.value = true
  try {
    const reply = await gachaMatchClient.ListDeckTemplates({
      status: -1,
      page: deckPage.value,
      pageSize: deckPageSize.value,
    })
    decks.value = reply.items ?? []
    deckTotal.value = toNumber(reply.total)
  } catch {
    message.error(t('gachaMatch.deckLoadFailed'))
  } finally {
    deckLoading.value = false
  }
}

function openDeckModal(record?: MemoryMatchDeckTemplate) {
  deckForm.id = record ? toNumber(record.id) : 0
  deckForm.name = record?.name ?? ''
  deckForm.status = record ? toNumber(record.status, 1) : 1
  deckForm.totalCards = record ? toNumber(record.totalCards, 54) : 54
  deckForm.weight = record ? toNumber(record.weight) : 0
  deckForm.configJson = record?.configJson || '{}'
  deckModalOpen.value = true
}

async function saveDeck() {
  if (!deckForm.name.trim()) {
    message.error(t('gachaMatch.nameRequired'))
    return
  }
  deckSaving.value = true
  try {
    await gachaMatchClient.SaveDeckTemplate({
      id: deckForm.id,
      name: deckForm.name.trim(),
      status: deckForm.status,
      totalCards: deckForm.totalCards,
      weight: deckForm.weight,
      configJson: deckForm.configJson,
    })
    message.success(t('gachaMatch.deckSaved'))
    deckModalOpen.value = false
    await loadDecks()
  } catch {
    // 错误提示由请求拦截器统一处理
  } finally {
    deckSaving.value = false
  }
}

async function deleteDeck(record: MemoryMatchDeckTemplate) {
  try {
    await gachaMatchClient.DeleteDeckTemplate(record.id ?? 0)
    message.success(t('gachaMatch.deckDeleted'))
    await loadDecks()
  } catch {
    // 错误提示由请求拦截器统一处理
  }
}

const appearances = ref<MemoryMatchAppearance[]>([])
const appearanceLoading = ref(false)
const appearancePage = ref(1)
const appearancePageSize = ref(20)
const appearanceTotal = ref(0)
const appearanceQuery = reactive({ type: -1, status: -1, keyword: '' })
const appearanceModalOpen = ref(false)
const appearanceSaving = ref(false)
const uploadingField = ref<'' | 'imageUrl' | 'backImageUrl'>('')
const appearanceForm = reactive({
  id: 0,
  type: 1,
  name: '',
  imageUrl: '',
  backImageUrl: '',
  status: 1,
  free: true,
  exchangePoints: 0,
  functionType: '',
  functionConfig: '',
  weight: 0,
})

const appearanceTypeOptions = computed(() => [
  { value: -1, label: t('gachaMatch.appearanceAllTypes') },
  { value: 1, label: t('gachaMatch.cardFace') },
  { value: 2, label: t('gachaMatch.cardBack') },
])
const appearanceTypeEditOptions = computed(() => [
  { value: 1, label: t('gachaMatch.cardFace') },
  { value: 2, label: t('gachaMatch.cardBack') },
])
const appearanceStatusOptions = computed(() => [
  { value: -1, label: t('gachaMatch.appearanceAllStatuses') },
  { value: 1, label: t('gachaMatch.enabled') },
  { value: 0, label: t('gachaMatch.disabled') },
])

const appearancePagination = computed(() => ({
  current: appearancePage.value,
  pageSize: appearancePageSize.value,
  total: appearanceTotal.value,
  showSizeChanger: true,
  showTotal: (total: number) => t('gachaMatch.totalItems', { total }),
  onChange: (page: number, pageSize: number) => {
    appearancePage.value = page
    appearancePageSize.value = pageSize
    void loadAppearances()
  },
}))

async function loadAppearances(page = appearancePage.value) {
  appearancePage.value = page
  appearanceLoading.value = true
  try {
    const reply = await gachaMatchClient.ListAppearances({
      type: appearanceQuery.type,
      status: appearanceQuery.status,
      keyword: appearanceQuery.keyword,
      page: appearancePage.value,
      pageSize: appearancePageSize.value,
    })
    appearances.value = reply.items ?? []
    appearanceTotal.value = toNumber(reply.total)
  } catch {
    message.error(t('gachaMatch.appearanceLoadFailed'))
  } finally {
    appearanceLoading.value = false
  }
}

function openAppearanceModal(record?: MemoryMatchAppearance) {
  appearanceForm.id = record ? toNumber(record.id) : 0
  appearanceForm.type = record ? toNumber(record.type, 1) : 1
  appearanceForm.name = record?.name ?? ''
  appearanceForm.imageUrl = record?.imageUrl ?? ''
  appearanceForm.backImageUrl = record?.backImageUrl ?? ''
  appearanceForm.status = record ? toNumber(record.status, 1) : 1
  appearanceForm.free = record ? toNumber(record.free, 1) === 1 : true
  appearanceForm.exchangePoints = record ? toNumber(record.exchangePoints) : 0
  appearanceForm.functionType = record?.functionType ?? ''
  appearanceForm.functionConfig = record?.functionConfig ?? ''
  appearanceForm.weight = record ? toNumber(record.weight) : 0
  appearanceModalOpen.value = true
}

async function uploadAppearanceImage(
  file: File,
  field: 'imageUrl' | 'backImageUrl',
): Promise<boolean> {
  if (!file.type.startsWith('image/')) {
    message.warning(t('gachaMatch.imageOnly'))
    return false
  }
  if (file.size > 10 * 1024 * 1024) {
    message.warning(t('gachaMatch.imageTooLarge'))
    return false
  }
  uploadingField.value = field
  try {
    const reply = await uploadFileToOss(file, 'gacha/match')
    if (reply.url) {
      appearanceForm[field] = reply.url
      message.success(t('gachaMatch.uploadSuccess'))
    }
  } catch {
    // 错误提示由请求拦截器统一处理
  } finally {
    uploadingField.value = ''
  }
  return false
}

async function saveAppearance() {
  if (!appearanceForm.name.trim()) {
    message.error(t('gachaMatch.nameRequired'))
    return
  }
  appearanceSaving.value = true
  try {
    await gachaMatchClient.SaveAppearance({
      id: appearanceForm.id,
      type: appearanceForm.type,
      name: appearanceForm.name.trim(),
      imageUrl: appearanceForm.imageUrl,
      backImageUrl: appearanceForm.backImageUrl,
      status: appearanceForm.status,
      free: appearanceForm.free ? 1 : 0,
      exchangePoints: appearanceForm.exchangePoints.toFixed(2),
      functionType: appearanceForm.functionType,
      functionConfig: appearanceForm.functionConfig,
      weight: appearanceForm.weight,
    })
    message.success(t('gachaMatch.appearanceSaved'))
    appearanceModalOpen.value = false
    await loadAppearances()
  } catch {
    // 错误提示由请求拦截器统一处理
  } finally {
    appearanceSaving.value = false
  }
}

async function deleteAppearance(record: MemoryMatchAppearance) {
  try {
    await gachaMatchClient.DeleteAppearance(record.id ?? 0)
    message.success(t('gachaMatch.appearanceDeleted'))
    await loadAppearances()
  } catch {
    // 错误提示由请求拦截器统一处理
  }
}

onMounted(() => {
  void loadRules()
  void loadDecks()
  void loadAppearances(1)
})
</script>

<style scoped lang="scss">
.gacha-match-page {
  min-height: calc(100vh - 132px);
  padding: 14px 16px 18px;
  background: #fff;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
}

.page-sub {
  margin: -8px 0 12px;
  color: #8c8c8c;
  font-size: 13px;
}

.match-tabs {
  :deep(.ant-tabs-nav) {
    margin-bottom: 18px;
  }
}

.rules-panel {
  max-width: 1160px;
  margin: 0 auto;
  padding: 8px 0 24px;
}

.rules-alert {
  margin-bottom: 24px;

  // 全局规则说明保持单行展示（窄屏允许横向溢出，不自动折行）
  :deep(.ant-alert-message) {
    white-space: nowrap;
  }
}

.rules-form {
  max-width: 760px;
  margin: 0 auto;
}

.number-input {
  width: 100%;
}

.rules-actions {
  display: flex;
  justify-content: center;
  margin-top: 8px;
}

.toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  margin-bottom: 14px;
}

.filter-select {
  width: 160px;
}

.filter-input {
  width: 240px;
}

.upload-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.upload-preview {
  margin-top: 8px;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
  object-fit: cover;
}
</style>
