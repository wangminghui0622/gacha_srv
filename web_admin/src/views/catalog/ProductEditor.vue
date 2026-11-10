<template>
  <a-card :title="$t('catalog.editorTitle')" style="margin-bottom: 16px">
    <a-alert :message="$t('catalog.editorAlert')" type="info" show-icon />
    <a-space wrap style="margin: 16px 0">
      <a-input v-model:value="lookupId" :placeholder="$t('catalog.productId')" :aria-label="$t('catalog.productId')" :disabled="busy" />
      <a-button v-if="can('read')" :disabled="busy" @click="read(false)">{{ $t('catalog.readActive') }}</a-button>
      <a-button v-if="can('read')" :disabled="busy" @click="read(true)">{{ $t('catalog.readRevision') }}</a-button>
      <a-button v-if="can('create')" :disabled="busy" @click="reset">{{ $t('common.create') }}</a-button>
    </a-space>
    <a-descriptions :column="2">
      <a-descriptions-item :label="$t('catalog.currentProduct')">{{ current?.productId ?? $t('common.create') }}</a-descriptions-item>
      <a-descriptions-item :label="$t('catalog.versionStatus')">{{ current?.version ?? $t('common.none') }} / {{ current?.status ? statusText(current.status) : $t('common.none') }}{{ revision ? $t('catalog.revisionSuffix') : '' }}</a-descriptions-item>
    </a-descriptions>
    <a-form layout="vertical">
      <a-form-item :label="$t('catalog.productCode')"><a-input v-model:value="code" :disabled="busy || !!current || !can('create')" /></a-form-item>
      <a-form-item :label="$t('common.name')"><a-input v-model:value="title" :disabled="busy || !editable" /></a-form-item>
      <a-form-item :label="$t('catalog.optionsJson')"><a-textarea v-model:value="optionsJson" :rows="6" :disabled="busy || !editable" /></a-form-item>
      <a-form-item :label="$t('catalog.skusJson')"><a-textarea v-model:value="skusJson" :rows="10" :disabled="busy || !editable" /></a-form-item>
    </a-form>
    <a-space wrap>
      <a-button v-if="can('create')" :loading="busy" :disabled="!!current" @click="create">{{ $t('catalog.createProduct') }}</a-button>
      <a-button v-if="can('write')" :loading="busy" :disabled="!revisable" @click="save(false)">{{ $t('catalog.saveMetaRevision') }}</a-button>
      <a-button v-if="can('write')" :loading="busy" :disabled="!revisable" @click="save(true)">{{ $t('catalog.saveVariantRevision') }}</a-button>
      <a-popconfirm v-if="can('publish')" :title="$t('catalog.publishConfirm')" @confirm="publish">
        <a-button :disabled="busy || current?.status !== 'PRODUCT_STATUS_DRAFT'">{{ $t('catalog.publishSaved') }}</a-button>
      </a-popconfirm>
    </a-space>
  </a-card>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { catalogClient as api } from '@/api/catalog'
import type { CreateProductInput as Input, Product } from '@/api/catalog'
import { allows } from '@/access/grants'
import { useSessionStore } from '@/stores/session'

const emit = defineEmits<{ changed: [] }>()
const { t } = useI18n()
const session = useSessionStore()
const can = (action: string) => allows(session.permissionNodes, 'catalog.product', action)
const statusKeys: Record<string, string> = { PRODUCT_STATUS_UNSPECIFIED: 'unspecified', PRODUCT_STATUS_DRAFT: 'draft', PRODUCT_STATUS_ACTIVE: 'active', PRODUCT_STATUS_ARCHIVED: 'archived' }
function statusText(status: string | number) {
  const key = statusKeys[String(status)]
  return key ? t(`catalog.statuses.${key}`) : t('common.none')
}
const current = ref<Product>()
const revision = ref(false)
const busy = ref(false)
const lookupId = ref('')
const baseVersion = ref('')
const code = ref('')
const title = ref('')
const optionsJson = ref('[]')
const skusJson = ref('[]')
const loadedOptions = ref('[]')
const loadedSelections = new Map<string, string>()
const revisable = computed(() => !!current.value && (current.value.status === 'PRODUCT_STATUS_ACTIVE' || revision.value))
const editable = computed(() => (current.value ? can('write') && revisable.value : can('create')))
function validId(value: string) {
  return /^[1-9]\d*$/.test(value) && BigInt(value) <= 18446744073709551615n
}
function reset() {
  current.value = undefined
  revision.value = false
  lookupId.value = ''
  baseVersion.value = ''
  code.value = ''
  title.value = ''
  optionsJson.value = JSON.stringify([{ name: '颜色', sortOrder: 0, values: [{ value: '红', sortOrder: 0 }, { value: '蓝', sortOrder: 1 }] }], null, 2)
  skusJson.value = JSON.stringify(
    ['红', '蓝'].map((name, index) => ({
      skuCode: `SKU-${index + 1}`,
      title: name,
      fulfillmentMode: 'FULFILLMENT_MODE_PHYSICAL',
      salePrice: { amountMinor: '100', currency: 'CNY' },
      referencePrice: { amountMinor: '100', currency: 'CNY' },
      selections: [{ optionIndex: 0, valueIndex: index }],
    })),
    null,
    2,
  )
}
function accept(value: Product, isRevision: boolean) {
  current.value = value
  revision.value = isRevision
  lookupId.value = value.productId ?? ''
  code.value = value.productCode ?? ''
  title.value = value.title ?? ''
  baseVersion.value = value.version && validId(value.version) ? (isRevision ? (BigInt(value.version) - 1n).toString() : value.version) : ''
  const options = value.options ?? []
  optionsJson.value = JSON.stringify(
    options.map((option) => ({ name: option.name, sortOrder: option.sortOrder, values: option.values?.map((item) => ({ value: item.value, sortOrder: item.sortOrder })) })),
    null,
    2,
  )
  skusJson.value = JSON.stringify(
    (value.skus ?? [])
      .filter((sku) => sku.status !== 'PRODUCT_STATUS_ARCHIVED')
      .map((sku) => ({
        skuCode: sku.skuCode,
        title: sku.title,
        fulfillmentMode: sku.fulfillmentMode,
        salePrice: sku.salePrice,
        referencePrice: sku.referencePrice,
        externalCode: sku.externalCode,
        selections: sku.selections?.map((selection) => {
          const index = options.findIndex((option) => option.optionId === selection.optionId)
          return { optionIndex: index, valueIndex: options[index]?.values?.findIndex((item) => item.optionValueId === selection.optionValueId) ?? -1 }
        }),
      })),
    null,
    2,
  )
  loadedOptions.value = JSON.stringify(JSON.parse(optionsJson.value))
  loadedSelections.clear()
  for (const sku of JSON.parse(skusJson.value) as NonNullable<Input['skus']>) loadedSelections.set(sku.skuCode ?? '', JSON.stringify(sku.selections))
}
async function run(task: () => Promise<void>, writing = false) {
  if (busy.value) return
  busy.value = true
  try {
    await task()
  } catch {
    if (writing) message.warning(t('common.operationUncertain'))
  } finally {
    busy.value = false
  }
}
function read(draft: boolean) {
  if (!validId(lookupId.value)) {
    message.warning(t('catalog.validation.invalidId'))
    return
  }
  void run(async () => accept(await (draft ? api.GetRevision({ productId: lookupId.value }) : api.Get({ productId: lookupId.value })), draft))
}
function input(): Input {
  if (!code.value.trim() || !title.value.trim()) throw new Error(t('catalog.validation.codeTitleRequired'))
  const options: Input['options'] = JSON.parse(optionsJson.value)
  const skus: Input['skus'] = JSON.parse(skusJson.value)
  if (!Array.isArray(options) || !Array.isArray(skus) || !skus.length) throw new Error(t('catalog.validation.arraysRequired'))
  for (const option of options) if (!option || typeof option !== 'object' || !Array.isArray(option.values) || !option.values.length) throw new Error(t('catalog.validation.optionValuesRequired'))
  for (const sku of skus) {
    if (!sku || typeof sku !== 'object') throw new Error(t('catalog.validation.skuObjectRequired'))
    for (const price of [sku.salePrice, sku.referencePrice]) {
      const amount = price?.amountMinor
      if (!price || typeof amount !== 'string' || !/^\d+$/.test(amount) || BigInt(amount) > 9223372036854775807n || !/^[A-Z]{3}$/.test(price.currency ?? '')) throw new Error(t('catalog.validation.priceInvalid'))
    }
    if (sku.salePrice?.currency !== sku.referencePrice?.currency) throw new Error(t('catalog.validation.currencyMismatch'))
    if (!Array.isArray(sku.selections) || sku.selections.length !== options.length) throw new Error(t('catalog.validation.selectionRequired'))
    const seen = new Set<number>()
    for (const selection of sku.selections) {
      const oi = selection?.optionIndex
      const vi = selection?.valueIndex
      if (typeof oi !== 'number' || typeof vi !== 'number' || !Number.isInteger(oi) || !Number.isInteger(vi) || oi < 0 || vi < 0 || !options[oi]?.values?.[vi] || seen.has(oi)) throw new Error(t('catalog.validation.selectionInvalid'))
      seen.add(oi)
    }
  }
  return { productCode: code.value, title: title.value, options, skus }
}
function create() {
  if (current.value || !can('create')) return
  let value: Input
  try {
    value = input()
  } catch (error) {
    message.warning(error instanceof Error ? error.message : t('catalog.validation.formatError'))
    return
  }
  void run(
    async () => {
      accept(await api.Create(value), false)
      emit('changed')
      message.success(t('catalog.created'))
    },
    true,
  )
}
function save(variants: boolean) {
  const product = current.value
  if (!can('write') || !revisable.value || !product?.productId || !validId(product.productId) || !validId(baseVersion.value)) {
    message.warning(t('catalog.validation.readFirst'))
    return
  }
  let value: Input
  try {
    value = input()
  } catch (error) {
    message.warning(error instanceof Error ? error.message : t('catalog.validation.formatError'))
    return
  }
  if (!variants) {
    if (JSON.stringify(value.options) !== loadedOptions.value || value.skus?.some((sku) => JSON.stringify(sku.selections) !== loadedSelections.get(sku.skuCode ?? ''))) {
      message.warning(t('catalog.validation.variantsChanged'))
      return
    }
    const existing = product.skus ?? []
    if (value.skus?.length !== existing.length || value.skus.some((sku) => !existing.some((old) => old.skuCode === sku.skuCode))) {
      message.warning(t('catalog.validation.skusKeptRequired'))
      return
    }
  }
  void run(
    async () => {
      const result = variants
        ? await api.SaveVariantRevision({ productId: product.productId!, expectedVersion: baseVersion.value, product: value })
        : await api.SaveRevision({
            productId: product.productId!,
            expectedVersion: baseVersion.value,
            title: value.title,
            skus: value.skus?.map((sku) => ({
              skuId: product.skus?.find((old) => old.skuCode === sku.skuCode)?.skuId ?? '',
              title: sku.title,
              externalCode: sku.externalCode,
              fulfillmentMode: sku.fulfillmentMode,
              salePrice: sku.salePrice,
              referencePrice: sku.referencePrice,
            })) ?? [],
          })
      accept(result, true)
      message.success(t('catalog.revisionSaved'))
    },
    true,
  )
}
function publish() {
  const product = current.value
  if (!can('publish') || product?.status !== 'PRODUCT_STATUS_DRAFT' || !product.productId || !validId(product.productId) || !validId(baseVersion.value)) return
  void run(
    async () => {
      accept(await (revision.value ? api.PublishRevision({ productId: product.productId!, expectedVersion: baseVersion.value }) : api.Publish({ productId: product.productId!, expectedVersion: baseVersion.value })), false)
      emit('changed')
      message.success(t('catalog.published'))
    },
    true,
  )
}
</script>
