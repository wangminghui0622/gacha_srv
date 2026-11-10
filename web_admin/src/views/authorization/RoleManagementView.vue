<template>
  <div>
    <PageHeader :title="$t('authorization.title')" />
    <a-row :gutter="16">
      <a-col :span="8">
        <a-card :title="$t('authorization.roles')" :loading="loading">
          <template #extra><a-button type="primary" @click="showCreate = true">{{ $t('authorization.newRole') }}</a-button></template>
          <a-list :data-source="roles" bordered>
            <template #renderItem="{ item }">
              <a-list-item class="role-row" :class="{ active: selected?.id === item.id }" @click="selectRole(item.id)">
                <a-list-item-meta :title="item.name" :description="item.code" />
                <a-tag :color="item.enabled ? 'green' : 'default'">{{ item.enabled ? $t('authorization.enabled') : $t('authorization.disabled') }}</a-tag>
              </a-list-item>
            </template>
          </a-list>
        </a-card>
      </a-col>
      <a-col :span="16">
        <a-card :title="$t('authorization.permissionConfig')" :loading="detailLoading">
          <a-empty v-if="!selected" :description="$t('authorization.selectRole')" />
          <template v-else>
            <a-form layout="inline" style="margin-bottom: 16px">
              <a-form-item :label="$t('authorization.roleName')"><a-input v-model:value="selected.name" /></a-form-item>
              <a-form-item><a-switch v-model:checked="selected.enabled" :checked-children="$t('authorization.enabled')" :un-checked-children="$t('authorization.disabled')" /></a-form-item>
              <a-form-item><a-button @click="saveRole">{{ $t('authorization.saveRole') }}</a-button></a-form-item>
            </a-form>
            <a-table :data-source="permissionRows" :columns="columns" row-key="key" :pagination="false" size="small">
              <template #bodyCell="{ column, record }">
                <template v-if="column.key === 'enabled'"><a-checkbox v-model:checked="record.enabled" /></template>
                <template v-else-if="column.key === 'scope'">
                  <a-select v-model:value="record.dataScope" :disabled="!record.enabled" style="width: 130px" :options="scopeOptions" />
                </template>
                <template v-else-if="column.key === 'approval'">
                  <a-select v-model:value="record.approvalMode" :disabled="!record.enabled" style="width: 120px" :options="approvalOptions" />
                </template>
              </template>
            </a-table>
            <div class="actions"><a-button type="primary" :loading="saving" @click="savePermissions">{{ $t('authorization.savePermissions') }}</a-button></div>
          </template>
        </a-card>
      </a-col>
    </a-row>
    <a-card :title="$t('authorization.roleMembers')" :loading="membersLoading" style="margin-top: 16px">
      <a-empty v-if="!selected" :description="$t('authorization.selectRole')" />
      <template v-else>
        <a-list :data-source="members" bordered size="small">
          <template #renderItem="{ item }">
            <a-list-item>
              <a-button type="link" @click="loadPrincipal(String(item.principalId))">{{ item.displayName || item.principalId }}</a-button>
              <span>{{ $t('authorization.principalId') }}：{{ item.principalId }}</span>
            </a-list-item>
          </template>
        </a-list>
        <div v-if="nextPrincipalId" class="actions"><a-button @click="loadMoreMembers">{{ $t('common.loadMore') }}</a-button></div>
        <a-divider />
        <a-form layout="inline">
          <a-form-item :label="$t('authorization.principalId')"><a-input v-model:value="principalIdInput" :placeholder="$t('authorization.principalIdPlaceholder')" /></a-form-item>
          <a-form-item><a-button :loading="principalLoading" @click="loadPrincipal(principalIdInput)">{{ $t('authorization.readRoles') }}</a-button></a-form-item>
        </a-form>
        <a-form v-if="principalRoles" layout="vertical" style="margin-top: 16px">
          <a-form-item :label="$t('authorization.principalRoles', { id: principalRoles.principalId })">
            <a-select v-model:value="assignedRoleIds" mode="multiple" style="width: 100%" :options="roleOptions" :placeholder="$t('authorization.selectRoles')" />
          </a-form-item>
          <a-button type="primary" :loading="principalSaving" @click="savePrincipalRoles">{{ $t('authorization.saveBindings') }}</a-button>
        </a-form>
      </template>
    </a-card>
    <a-modal v-model:open="showCreate" :title="$t('authorization.newRole')" :confirm-loading="creating" @ok="createRole">
      <a-form layout="vertical">
        <a-form-item :label="$t('authorization.roleCode')" required><a-input v-model:value="draft.code" :placeholder="$t('authorization.codePlaceholder')" /></a-form-item>
        <a-form-item :label="$t('authorization.roleNameRequired')" required><a-input v-model:value="draft.name" /></a-form-item>
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
  createConsoleRole,
  fetchPermissionCatalog,
  getConsolePrincipalRoles,
  getConsoleRole,
  listConsoleRoleMembers,
  listConsoleRoles,
  putConsolePrincipalRoles,
  putConsoleRolePermissions,
  updateConsoleRole,
  type ConsolePrincipalRoles,
  type ConsoleRole,
  type ConsoleRoleMember,
  type PermissionCatalogResource,
  type RolePermission,
} from '@/api/console'

interface PermissionRow {
  key: string
  resource: string
  resourceName: string
  action: string
  actionName: string
  enabled: boolean
  dataScope: number
  approvalMode: number
}
const { t } = useI18n()
const columns = computed(() => [
  { title: t('authorization.columns.resource'), dataIndex: 'resourceName' },
  { title: t('authorization.columns.action'), dataIndex: 'actionName' },
  { title: t('authorization.columns.grant'), key: 'enabled', width: 80 },
  { title: t('authorization.columns.scope'), key: 'scope', width: 160 },
  { title: t('authorization.columns.approval'), key: 'approval', width: 150 },
])
const scopeOptions = computed(() => [
  { value: 1, label: t('authorization.scopes.self') },
  { value: 2, label: t('authorization.scopes.team') },
  { value: 3, label: t('authorization.scopes.department') },
  { value: 4, label: t('authorization.scopes.organization') },
  { value: 5, label: t('authorization.scopes.custom') },
  { value: 6, label: t('authorization.scopes.all') },
])
const approvalOptions = computed(() => [
  { value: 1, label: t('authorization.approvals.none') },
  { value: 2, label: t('authorization.approvals.single') },
  { value: 3, label: t('authorization.approvals.dual') },
])
const loading = ref(false)
const detailLoading = ref(false)
const saving = ref(false)
const creating = ref(false)
const showCreate = ref(false)
const roles = ref<ConsoleRole[]>([])
const catalog = ref<PermissionCatalogResource[]>([])
const selected = ref<ConsoleRole | null>(null)
const draft = reactive({ code: '', name: '' })
const rows = ref<PermissionRow[]>([])
const permissionRows = computed(() => rows.value)
const membersLoading = ref(false)
const principalLoading = ref(false)
const principalSaving = ref(false)
const members = ref<ConsoleRoleMember[]>([])
const nextPrincipalId = ref('')
const principalIdInput = ref('')
const principalRoles = ref<ConsolePrincipalRoles | null>(null)
const assignedRoleIds = ref<string[]>([])
const roleOptions = computed(() => roles.value.filter((role) => role.enabled).map((role) => ({ value: role.id, label: `${role.name} (${role.code})` })))

function buildRows(role: ConsoleRole) {
  const owned = new Map((role.permissions ?? []).map((item) => [`${item.resource}:${item.action}`, item]))
  rows.value = catalog.value.flatMap((resource) =>
    (resource.actions ?? []).map((action) => {
      const permission = owned.get(`${resource.code}:${action.code}`)
      return {
        key: `${resource.code}:${action.code}`,
        resource: resource.code,
        resourceName: resource.name,
        action: action.code,
        actionName: action.name,
        enabled: Boolean(permission),
        dataScope: permission?.dataScope ?? 6,
        approvalMode: permission?.approvalMode ?? 1,
      }
    }),
  )
}
async function reloadRoles() {
  loading.value = true
  try {
    roles.value = (await listConsoleRoles()).roles ?? []
  } finally {
    loading.value = false
  }
}
async function selectRole(id: string) {
  detailLoading.value = true
  try {
    selected.value = await getConsoleRole(id)
    buildRows(selected.value)
    principalRoles.value = null
    await reloadMembers()
  } finally {
    detailLoading.value = false
  }
}
async function reloadMembers() {
  if (!selected.value) return
  membersLoading.value = true
  try {
    const page = await listConsoleRoleMembers(selected.value.id)
    members.value = page.members ?? []
    nextPrincipalId.value = page.nextPrincipalId === '0' ? '' : (page.nextPrincipalId ?? '')
  } finally {
    membersLoading.value = false
  }
}
async function loadMoreMembers() {
  if (!selected.value || !nextPrincipalId.value) return
  membersLoading.value = true
  try {
    const page = await listConsoleRoleMembers(selected.value.id, nextPrincipalId.value)
    members.value.push(...(page.members ?? []))
    nextPrincipalId.value = page.nextPrincipalId === '0' ? '' : (page.nextPrincipalId ?? '')
  } finally {
    membersLoading.value = false
  }
}
async function loadPrincipal(id: string) {
  if (!/^\d+$/.test(id) || id === '0') {
    message.warning(t('authorization.validation.invalidPrincipalId'))
    return
  }
  principalLoading.value = true
  try {
    principalRoles.value = await getConsolePrincipalRoles(id)
    principalIdInput.value = id
    assignedRoleIds.value = [...(principalRoles.value.roleIds ?? [])]
  } catch {
    message.error(t('authorization.loadPrincipalFailed'))
  } finally {
    principalLoading.value = false
  }
}
async function savePrincipalRoles() {
  if (!principalRoles.value) return
  principalSaving.value = true
  try {
    principalRoles.value = await putConsolePrincipalRoles(principalRoles.value.principalId, principalRoles.value.version, assignedRoleIds.value)
    assignedRoleIds.value = [...(principalRoles.value.roleIds ?? [])]
    await reloadMembers()
    message.success(t('authorization.bindingsSaved'))
  } catch {
    message.error(t('authorization.saveFailed'))
  } finally {
    principalSaving.value = false
  }
}
async function createRole() {
  if (!draft.code.trim() || !draft.name.trim()) {
    message.warning(t('authorization.validation.codeNameRequired'))
    return
  }
  creating.value = true
  try {
    const role = await createConsoleRole({ code: draft.code.trim(), name: draft.name.trim() })
    showCreate.value = false
    draft.code = ''
    draft.name = ''
    await reloadRoles()
    await selectRole(role.id)
    message.success(t('authorization.roleCreated'))
  } finally {
    creating.value = false
  }
}
async function saveRole() {
  if (!selected.value) return
  selected.value = await updateConsoleRole(selected.value.id, { name: selected.value.name, enabled: selected.value.enabled, expectedVersion: selected.value.version })
  buildRows(selected.value)
  await reloadRoles()
  message.success(t('authorization.roleSaved'))
}
async function savePermissions() {
  if (!selected.value) return
  const permissions: RolePermission[] = rows.value.filter((row) => row.enabled).map((row) => ({ resource: row.resource, action: row.action, dataScope: row.dataScope, approvalMode: row.approvalMode }))
  saving.value = true
  try {
    selected.value = await putConsoleRolePermissions(selected.value.id, selected.value.version, permissions)
    buildRows(selected.value)
    await reloadRoles()
    message.success(t('authorization.permissionsPublished'))
  } finally {
    saving.value = false
  }
}
onMounted(async () => {
  const [tree] = await Promise.all([fetchPermissionCatalog(), reloadRoles()])
  catalog.value = tree.resources ?? []
  if (roles.value[0]) await selectRole(roles.value[0].id)
})
</script>

<style scoped>
.role-row { cursor: pointer; }
.role-row.active { background: #e6f4ff; }
.actions { margin-top: 16px; text-align: right; }
</style>
