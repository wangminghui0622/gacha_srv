<template>
  <div>
    <PageHeader :title="$t('nav.system')" :show-back="false" />
    <p class="intro">{{ $t('system.intro') }}</p>
    <a-row :gutter="[16, 16]">
      <a-col v-for="card in cards" :key="card.path" :xs="24" :sm="12" :xl="8">
        <a-card hoverable class="entry-card" @click="router.push(card.path)">
          <a-space align="start" :size="14">
            <component :is="card.icon" class="entry-icon" />
            <div>
              <div class="entry-title">{{ $t(card.title) }}</div>
              <div class="entry-desc">{{ $t(card.desc) }}</div>
            </div>
          </a-space>
        </a-card>
      </a-col>
    </a-row>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import {
  FilterOutlined,
  FlagOutlined,
  GiftOutlined,
  LineChartOutlined,
  SafetyOutlined,
  UnorderedListOutlined,
} from '@ant-design/icons-vue'
import PageHeader from '@/components/PageHeader.vue'
import { allows } from '@/access/grants'
import { useSessionStore } from '@/stores/session'

const router = useRouter()
const session = useSessionStore()

const allCards = [
  {
    path: '/operations/task-plans',
    title: 'nav.taskPlan',
    desc: 'system.taskPlanDesc',
    icon: UnorderedListOutlined,
    resource: 'task.plan',
    action: 'read',
  },
  {
    path: '/operations/tasks',
    title: 'nav.taskProgress',
    desc: 'system.taskProgressDesc',
    icon: FlagOutlined,
    resource: 'task.progress',
    action: 'read',
  },
  {
    path: '/eligibility',
    title: 'nav.eligibility',
    desc: 'system.eligibilityDesc',
    icon: FilterOutlined,
    resource: 'eligibility.policy',
    action: 'read',
  },
  {
    path: '/risk',
    title: 'nav.risk',
    desc: 'system.riskDesc',
    icon: SafetyOutlined,
    resource: 'risk.case',
    action: 'list',
  },
  {
    path: '/reporting',
    title: 'nav.reporting',
    desc: 'system.reportingDesc',
    icon: LineChartOutlined,
    resource: 'reporting.metric',
    action: 'read',
  },
  {
    path: '/promotion',
    title: 'nav.promotion',
    desc: 'system.promotionDesc',
    icon: GiftOutlined,
    resource: 'promotion.coupon_template',
    action: 'read',
  },
]

const cards = computed(() =>
  allCards.filter((card) => allows(session.permissionNodes, card.resource, card.action)),
)
</script>

<style scoped lang="scss">
.intro {
  color: rgba(0, 0, 0, 0.55);
  margin: 0 0 16px;
}

.entry-card {
  cursor: pointer;
  min-height: 96px;
}

.entry-icon {
  color: #1677ff;
  font-size: 24px;
  margin-top: 2px;
}

.entry-title {
  font-size: 15px;
  font-weight: 600;
  margin-bottom: 4px;
}

.entry-desc {
  color: rgba(0, 0, 0, 0.55);
  line-height: 1.5;
}
</style>
