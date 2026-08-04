<template>
  <div class="data-charts-page">
    <div class="charts-header">
      <h2 class="charts-title">{{ t('menu.dataCharts') }}</h2>
      <p class="charts-desc">{{ t('dataCharts.desc') }}</p>
    </div>

    <!-- 顶部统计卡片 -->
    <div class="stats-cards">
      <div class="stat-card" v-for="card in statCards" :key="card.label">
        <div class="stat-icon" :style="{ background: card.bg }">
          <span class="stat-emoji">{{ card.emoji }}</span>
        </div>
        <div class="stat-info">
          <span class="stat-value">{{ card.value }}</span>
          <span class="stat-label">{{ card.label }}</span>
        </div>
      </div>
    </div>

    <!-- 图表网格 -->
    <div class="charts-grid">
      <!-- 组织层级树图 -->
      <div class="chart-card chart-card--wide">
        <div class="chart-card-header">
          <h3>{{ t('dataCharts.orgTree') }}</h3>
          <t-radio-group v-model="orgTreeMode" variant="default-filled" size="small">
            <t-radio-button value="tree">{{ t('dataCharts.treeView') }}</t-radio-button>
            <t-radio-button value="treemap">{{ t('dataCharts.treemapView') }}</t-radio-button>
          </t-radio-group>
        </div>
        <div class="chart-container chart-container--tall">
          <v-chart v-if="orgTreeOption" :option="orgTreeOption" autoresize />
          <t-empty v-else :description="t('dataCharts.noData')" />
        </div>
      </div>

      <!-- 知识库文档数量 Top 10 -->
      <div class="chart-card">
        <div class="chart-card-header">
          <h3>{{ t('dataCharts.kbKnowledgeCount') }}</h3>
        </div>
        <div class="chart-container">
          <v-chart v-if="kbKnowledgeOption" :option="kbKnowledgeOption" autoresize />
          <t-empty v-else :description="t('dataCharts.noData')" />
        </div>
      </div>

      <!-- 知识库切片数量 Top 10 -->
      <div class="chart-card">
        <div class="chart-card-header">
          <h3>{{ t('dataCharts.kbChunkCount') }}</h3>
        </div>
        <div class="chart-container">
          <v-chart v-if="kbChunkOption" :option="kbChunkOption" autoresize />
          <t-empty v-else :description="t('dataCharts.noData')" />
        </div>
      </div>

      <!-- 知识库类型分布 -->
      <div class="chart-card">
        <div class="chart-card-header">
          <h3>{{ t('dataCharts.kbTypeDist') }}</h3>
        </div>
        <div class="chart-container">
          <v-chart v-if="kbTypeOption" :option="kbTypeOption" autoresize />
          <t-empty v-else :description="t('dataCharts.noData')" />
        </div>
      </div>

      <!-- 智能体模式分布 -->
      <div class="chart-card">
        <div class="chart-card-header">
          <h3>{{ t('dataCharts.agentModeDist') }}</h3>
        </div>
        <div class="chart-container">
          <v-chart v-if="agentModeOption" :option="agentModeOption" autoresize />
          <t-empty v-else :description="t('dataCharts.noData')" />
        </div>
      </div>

      <!-- 智能体知识库数量 -->
      <div class="chart-card">
        <div class="chart-card-header">
          <h3>{{ t('dataCharts.agentKBCount') }}</h3>
        </div>
        <div class="chart-container">
          <v-chart v-if="agentKBOption" :option="agentKBOption" autoresize />
          <t-empty v-else :description="t('dataCharts.noData')" />
        </div>
      </div>

      <!-- 智能体功能使用统计 -->
      <div class="chart-card">
        <div class="chart-card-header">
          <h3>{{ t('dataCharts.agentFeatureDist') }}</h3>
        </div>
        <div class="chart-container">
          <v-chart v-if="agentFeatureOption" :option="agentFeatureOption" autoresize />
          <t-empty v-else :description="t('dataCharts.noData')" />
        </div>
      </div>

      <!-- 成员分布柱状图 -->
      <div class="chart-card">
        <div class="chart-card-header">
          <h3>{{ t('dataCharts.memberDist') }}</h3>
        </div>
        <div class="chart-container">
          <v-chart v-if="memberBarOption" :option="memberBarOption" autoresize />
          <t-empty v-else :description="t('dataCharts.noData')" />
        </div>
      </div>

      <!-- 组织资源雷达图 -->
      <div class="chart-card">
        <div class="chart-card-header">
          <h3>{{ t('dataCharts.resourceRadar') }}</h3>
        </div>
        <div class="chart-container">
          <v-chart v-if="radarOption" :option="radarOption" autoresize />
          <t-empty v-else :description="t('dataCharts.noData')" />
        </div>
      </div>

      <!-- 共享资源热力图 -->
      <div class="chart-card chart-card--wide">
        <div class="chart-card-header">
          <h3>{{ t('dataCharts.shareHeatmap') }}</h3>
        </div>
        <div class="chart-container">
          <v-chart v-if="shareOption" :option="shareOption" autoresize />
          <t-empty v-else :description="t('dataCharts.noData')" />
        </div>
      </div>
    </div>

    <t-loading :loading="loading" fullscreen />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import VChart from 'vue-echarts'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { BarChart, PieChart, TreeChart, TreemapChart, RadarChart, HeatmapChart } from 'echarts/charts'
import {
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  GridComponent,
  RadarComponent,
  ToolboxComponent,
  DataZoomComponent,
} from 'echarts/components'
import { listOrgUnits, type OrgUnit } from '@/api/org-unit'
import { listMyOrganizations, type Organization, type ResourceCountsByOrg } from '@/api/organization'
import { listKnowledgeBases } from '@/api/knowledge-base'
import { listAgents } from '@/api/agent'

// ── Theme detection (ECharts canvas does not support CSS variables) ──
const isDark = ref(document.documentElement.getAttribute('theme-mode') === 'dark')
const themeObserver = new MutationObserver(() => {
  isDark.value = document.documentElement.getAttribute('theme-mode') === 'dark'
})
themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['theme-mode'] })
onUnmounted(() => themeObserver.disconnect())

// Theme-aware colors for ECharts
const T = computed(() => isDark.value ? {
  text: '#e8e8e8',
  textSec: '#999',
  border: '#3a3a3a',
  bgPage: '#1a1a2e',
  splitLine: '#333',
  heatmapRange: ['#0d2818', '#14532d', '#15803d', '#22c55e', '#4ade80'],
} : {
  text: '#1a1a2e',
  textSec: '#666',
  border: '#e8e8e8',
  bgPage: '#f5f5f5',
  splitLine: '#e8e8e8',
  heatmapRange: ['#f0fdf4', '#86efac', '#22c55e', '#15803d', '#14532d'],
})

// Register ECharts components
use([
  CanvasRenderer,
  BarChart,
  PieChart,
  TreeChart,
  TreemapChart,
  RadarChart,
  HeatmapChart,
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  GridComponent,
  RadarComponent,
  ToolboxComponent,
  DataZoomComponent,
])

const { t } = useI18n()

const loading = ref(true)
const orgTree = ref<OrgUnit[]>([])
const organizations = ref<Organization[]>([])
const resourceCounts = ref<ResourceCountsByOrg | null>(null)
const knowledgeBases = ref<any[]>([])
const agents = ref<any[]>([])
const orgTreeMode = ref<'tree' | 'treemap'>('tree')

// ── Color palette ──
const COLORS = ['#07C05F', '#00B2FF', '#FF6B35', '#A855F7', '#F59E0B', '#EC4899', '#06B6D4', '#8B5CF6', '#10B981', '#F43F5E']

// ── Fetch data ──
onMounted(async () => {
  try {
    const [treeData, orgsResp, kbResp, agentResp] = await Promise.all([
      listOrgUnits(true),
      listMyOrganizations(),
      listKnowledgeBases().catch(() => ({ data: [] })),
      listAgents({ purpose: 'manage' }).catch(() => ({ data: [] })),
    ])
    orgTree.value = treeData || []
    organizations.value = orgsResp?.data?.organizations || []
    resourceCounts.value = orgsResp?.data?.resource_counts || null
    // listKnowledgeBases returns { data: [...] } or the array directly
    const kbData = kbResp?.data ?? kbResp ?? []
    knowledgeBases.value = Array.isArray(kbData) ? kbData : (kbData as any)?.items || []
    // listAgents returns { data: CustomAgent[] }; filter built-in agents
    const agentData = agentResp?.data ?? agentResp ?? []
    const rawAgents = Array.isArray(agentData) ? agentData : (agentData as any)?.items || []
    agents.value = rawAgents.filter((a: any) => !a.is_builtin && !a.id?.startsWith('builtin-'))
  } catch (e) {
    console.error('Failed to load chart data:', e)
  } finally {
    loading.value = false
  }
})

// ── Flatten org tree ──
function flattenTree(nodes: OrgUnit[], depth = 0): Array<OrgUnit & { _depth: number }> {
  const result: Array<OrgUnit & { _depth: number }> = []
  for (const node of nodes) {
    result.push({ ...node, _depth: depth })
    if (node.children?.length) {
      result.push(...flattenTree(node.children, depth + 1))
    }
  }
  return result
}

const flatUnits = computed(() => flattenTree(orgTree.value))

// ── Summary stats ──
const totalOrgUnits = computed(() => flatUnits.value.length)
const totalOrgs = computed(() => organizations.value.length)
const totalMembers = computed(() =>
  organizations.value.reduce((sum, org) => sum + (org.member_count || 0), 0)
)
const totalShares = computed(() =>
  organizations.value.reduce((sum, org) => sum + (org.share_count || 0) + (org.agent_share_count || 0), 0)
)
const totalKBs = computed(() => knowledgeBases.value.length)
const totalAgents = computed(() => agents.value.length)
const totalKnowledge = computed(() =>
  knowledgeBases.value.reduce((sum: number, kb: any) => sum + (kb.knowledge_count || 0), 0)
)
const totalChunks = computed(() =>
  knowledgeBases.value.reduce((sum: number, kb: any) => sum + (kb.chunk_count || 0), 0)
)

const statCards = computed(() => [
  {
    emoji: '📚',
    label: t('dataCharts.knowledgeBases'),
    value: totalKBs.value,
    bg: 'linear-gradient(135deg, #00B2FF22, #00B2FF08)',
  },
  {
    emoji: '📄',
    label: t('dataCharts.totalKnowledge'),
    value: totalKnowledge.value,
    bg: 'linear-gradient(135deg, #07C05F22, #07C05F08)',
  },
  {
    emoji: '🧩',
    label: t('dataCharts.totalChunks'),
    value: totalChunks.value,
    bg: 'linear-gradient(135deg, #06B6D422, #06B6D408)',
  },
  {
    emoji: '🤖',
    label: t('dataCharts.agentCount'),
    value: totalAgents.value,
    bg: 'linear-gradient(135deg, #A855F722, #A855F708)',
  },
  {
    emoji: '👥',
    label: t('dataCharts.members'),
    value: totalMembers.value,
    bg: 'linear-gradient(135deg, #F59E0B22, #F59E0B08)',
  },
  {
    emoji: '🔗',
    label: t('dataCharts.shares'),
    value: totalShares.value,
    bg: 'linear-gradient(135deg, #FF6B3522, #FF6B3508)',
  },
])

// ── Org tree chart ──
function buildTreeData(nodes: OrgUnit[]): any[] {
  return nodes.map(node => ({
    name: node.name,
    value: node.children?.length || 0,
    children: node.children?.length ? buildTreeData(node.children) : undefined,
  }))
}

function buildTreemapData(nodes: OrgUnit[]): any[] {
  return nodes.map(node => ({
    name: node.name,
    value: node.children?.length ? undefined : 1,
    children: node.children?.length ? buildTreemapData(node.children) : undefined,
  }))
}

const orgTreeOption = computed(() => {
  if (!orgTree.value.length) return null

  if (orgTreeMode.value === 'tree') {
    return {
      tooltip: { trigger: 'item', triggerOn: 'mousemove' },
      series: [{
        type: 'tree',
        data: buildTreeData(orgTree.value),
        top: '5%',
        left: '15%',
        bottom: '5%',
        right: '20%',
        symbolSize: 12,
        orient: 'LR',
        label: {
          position: 'left',
          verticalAlign: 'middle',
          align: 'right',
          fontSize: 12,
          color: T.value.text,
        },
        leaves: {
          label: {
            position: 'right',
            verticalAlign: 'middle',
            align: 'left',
          },
        },
        lineStyle: { color: '#07C05F', width: 1.5, curveness: 0.5 },
        itemStyle: { color: '#07C05F', borderWidth: 0 },
        expandAndCollapse: true,
        initialTreeDepth: 3,
        animationDuration: 550,
        animationDurationUpdate: 750,
      }],
    }
  }

  return {
    tooltip: { trigger: 'item', formatter: '{b}: {c} 个子节点' },
    series: [{
      type: 'treemap',
      data: buildTreemapData(orgTree.value),
      roam: false,
      nodeClick: false,
      breadcrumb: { show: true, bottom: 0, textStyle: { color: T.value.text } },
      levels: [
        { itemStyle: { borderColor: T.value.border, borderWidth: 2, gapWidth: 2 } },
        { colorSaturation: [0.3, 0.6], itemStyle: { borderColorSaturation: 0.7, gapWidth: 1 } },
        { colorSaturation: [0.3, 0.5], itemStyle: { borderColorSaturation: 0.6, gapWidth: 1 } },
      ],
      label: { show: true, formatter: '{b}', fontSize: 12, color: T.value.text, textShadowColor: 'rgba(255,255,255,0.8)', textShadowBlur: 3 },
      itemStyle: { borderColor: T.value.border },
    }],
  }
})

// ── KB Knowledge Count Top 10 ──
const kbKnowledgeOption = computed(() => {
  if (!knowledgeBases.value.length) return null
  const sorted = [...knowledgeBases.value]
    .sort((a: any, b: any) => (b.knowledge_count || 0) - (a.knowledge_count || 0))
    .slice(0, 10)
  const names = sorted.map((kb: any) => kb.name?.length > 12 ? kb.name.slice(0, 12) + '…' : kb.name)
  const values = sorted.map((kb: any) => kb.knowledge_count || 0)

  return {
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    grid: { left: '3%', right: '4%', bottom: '8%', top: '8%', containLabel: true },
    xAxis: {
      type: 'value',
      minInterval: 1,
      axisLabel: { color: T.value.textSec },
      splitLine: { lineStyle: { color: T.value.border, type: 'dashed' } },
    },
    yAxis: {
      type: 'category',
      data: names.reverse(),
      axisLabel: { fontSize: 11, color: T.value.textSec },
      axisLine: { lineStyle: { color: T.value.border } },
    },
    series: [{
      type: 'bar',
      data: values.reverse(),
      barMaxWidth: 24,
      itemStyle: {
        borderRadius: [0, 4, 4, 0],
        color: {
          type: 'linear', x: 0, y: 0, x2: 1, y2: 0,
          colorStops: [
            { offset: 0, color: '#07C05F44' },
            { offset: 1, color: '#07C05F' },
          ],
        },
      },
      label: { show: true, position: 'right', fontSize: 10, color: T.value.textSec },
    }],
  }
})

// ── KB Chunk Count Top 10 ──
const kbChunkOption = computed(() => {
  if (!knowledgeBases.value.length) return null
  const sorted = [...knowledgeBases.value]
    .sort((a: any, b: any) => (b.chunk_count || 0) - (a.chunk_count || 0))
    .slice(0, 10)
  const names = sorted.map((kb: any) => kb.name?.length > 12 ? kb.name.slice(0, 12) + '…' : kb.name)
  const values = sorted.map((kb: any) => kb.chunk_count || 0)

  return {
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    grid: { left: '3%', right: '4%', bottom: '8%', top: '8%', containLabel: true },
    xAxis: {
      type: 'value',
      minInterval: 1,
      axisLabel: { color: T.value.textSec },
      splitLine: { lineStyle: { color: T.value.border, type: 'dashed' } },
    },
    yAxis: {
      type: 'category',
      data: names.reverse(),
      axisLabel: { fontSize: 11, color: T.value.textSec },
      axisLine: { lineStyle: { color: T.value.border } },
    },
    series: [{
      type: 'bar',
      data: values.reverse(),
      barMaxWidth: 24,
      itemStyle: {
        borderRadius: [0, 4, 4, 0],
        color: {
          type: 'linear', x: 0, y: 0, x2: 1, y2: 0,
          colorStops: [
            { offset: 0, color: '#00B2FF44' },
            { offset: 1, color: '#00B2FF' },
          ],
        },
      },
      label: { show: true, position: 'right', fontSize: 10, color: T.value.textSec },
    }],
  }
})

// ── KB Type Distribution (document vs FAQ) ──
const kbTypeOption = computed(() => {
  if (!knowledgeBases.value.length) return null
  let docCount = 0
  let faqCount = 0
  let wikiCount = 0
  for (const kb of knowledgeBases.value) {
    const type = (kb as any).type || 'document'
    if (type === 'faq') faqCount++
    else if (type === 'wiki') wikiCount++
    else docCount++
  }
  const data = [
    { name: t('dataCharts.kbTypeDocument'), value: docCount },
    faqCount > 0 ? { name: t('dataCharts.kbTypeFAQ'), value: faqCount } : null,
    wikiCount > 0 ? { name: t('dataCharts.kbTypeWiki'), value: wikiCount } : null,
  ].filter(Boolean)

  return {
    tooltip: { trigger: 'item', formatter: '{b}: {c} ({d}%)' },
    legend: {
      bottom: '5%',
      left: 'center',
      textStyle: { color: T.value.textSec, fontSize: 11 },
    },
    series: [{
      type: 'pie',
      radius: ['40%', '70%'],
      center: ['50%', '42%'],
      avoidLabelOverlap: true,
      itemStyle: { borderRadius: 6, borderColor: T.value.bgPage, borderWidth: 2 },
      label: { show: true, formatter: '{b}\n{c}个', fontSize: 11, color: T.value.text },
      emphasis: {
        label: { show: true, fontSize: 14, fontWeight: 'bold' },
        itemStyle: { shadowBlur: 10, shadowOffsetX: 0, shadowColor: 'rgba(0, 0, 0, 0.5)' },
      },
      data,
      color: ['#00B2FF', '#FF6B35', '#A855F7'],
    }],
  }
})

// ── Agent Mode Distribution ──
const agentModeOption = computed(() => {
  if (!agents.value.length) return null
  let quickCount = 0
  let smartCount = 0
  let otherCount = 0
  for (const agent of agents.value) {
    const mode = (agent as any).config?.agent_mode || 'quick-answer'
    if (mode === 'smart-reasoning') smartCount++
    else if (mode === 'quick-answer') quickCount++
    else otherCount++
  }
  const data = [
    { name: t('dataCharts.agentModeQuick'), value: quickCount },
    { name: t('dataCharts.agentModeSmart'), value: smartCount },
    otherCount > 0 ? { name: t('dataCharts.agentModeOther'), value: otherCount } : null,
  ].filter(Boolean)

  return {
    tooltip: { trigger: 'item', formatter: '{b}: {c} ({d}%)' },
    legend: {
      bottom: '5%',
      left: 'center',
      textStyle: { color: T.value.textSec, fontSize: 11 },
    },
    series: [{
      type: 'pie',
      radius: ['40%', '70%'],
      center: ['50%', '42%'],
      avoidLabelOverlap: true,
      itemStyle: { borderRadius: 6, borderColor: T.value.bgPage, borderWidth: 2 },
      label: { show: true, formatter: '{b}\n{c}个', fontSize: 11, color: T.value.text },
      emphasis: {
        label: { show: true, fontSize: 14, fontWeight: 'bold' },
      },
      data,
      color: ['#07C05F', '#A855F7', '#6B7280'],
    }],
  }
})

// ── Agent KB Count ──
const agentKBOption = computed(() => {
  if (!agents.value.length) return null
  const sorted = [...agents.value]
    .filter((a: any) => a.config?.knowledge_bases?.length || a.config?.kb_selection_mode === 'all')
    .sort((a: any, b: any) => {
      const aLen = a.config?.kb_selection_mode === 'all' ? 999 : (a.config?.knowledge_bases?.length || 0)
      const bLen = b.config?.kb_selection_mode === 'all' ? 999 : (b.config?.knowledge_bases?.length || 0)
      return bLen - aLen
    })
    .slice(0, 10)

  if (!sorted.length) return null

  const names = sorted.map((a: any) => a.name?.length > 10 ? a.name.slice(0, 10) + '…' : a.name)
  const values = sorted.map((a: any) =>
    a.config?.kb_selection_mode === 'all' ? 'ALL' : (a.config?.knowledge_bases?.length || 0)
  )

  return {
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      formatter: (params: any) => {
        const p = params[0]
        return `${p.name}: ${p.value === 'ALL' ? t('dataCharts.kbModeAll') : p.value + ' ' + t('dataCharts.kbCount')}`
      },
    },
    grid: { left: '3%', right: '8%', bottom: '8%', top: '8%', containLabel: true },
    xAxis: {
      type: 'value',
      axisLabel: {
        color: T.value.textSec,
        formatter: (v: any) => v === 999 ? 'ALL' : v,
      },
      splitLine: { lineStyle: { color: T.value.border, type: 'dashed' } },
    },
    yAxis: {
      type: 'category',
      data: names.reverse(),
      axisLabel: { fontSize: 11, color: T.value.textSec },
      axisLine: { lineStyle: { color: T.value.border } },
    },
    series: [{
      type: 'bar',
      data: values.map((v: any) => v === 'ALL' ? 999 : v).reverse(),
      barMaxWidth: 24,
      itemStyle: {
        borderRadius: [0, 4, 4, 0],
        color: (params: any) => params.value === 999 ? '#EC4899' : '#A855F7',
      },
      label: {
        show: true,
        position: 'right',
        fontSize: 10,
        color: T.value.textSec,
        formatter: (params: any) => params.value === 999 ? 'ALL' : params.value,
      },
    }],
  }
})

// ── Agent Feature Usage ──
const agentFeatureOption = computed(() => {
  if (!agents.value.length) return null
  let webSearchCount = 0
  let multiTurnCount = 0
  let mcpCount = 0
  let imageCount = 0
  for (const agent of agents.value) {
    const cfg = (agent as any).config || {}
    if (cfg.web_search_enabled) webSearchCount++
    if (cfg.multi_turn_enabled) multiTurnCount++
    if (cfg.mcp_selection_mode === 'selected' && cfg.mcp_services?.length) mcpCount++
    if (cfg.image_vlm_enabled || cfg.image_understanding_enabled) imageCount++
  }

  const categories = [
    t('dataCharts.featureWebSearch'),
    t('dataCharts.featureMultiTurn'),
    t('dataCharts.featureMCP'),
    t('dataCharts.featureImage'),
  ]
  const values = [webSearchCount, multiTurnCount, mcpCount, imageCount]

  return {
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    radar: {
      indicator: categories.map(c => ({ name: c, max: agents.value.length })),
      center: ['50%', '52%'],
      radius: '60%',
      axisName: { color: T.value.textSec, fontSize: 11 },
      splitArea: { areaStyle: { color: ['transparent'] } },
      splitLine: { lineStyle: { color: T.value.border } },
      axisLine: { lineStyle: { color: T.value.border } },
    },
    series: [{
      type: 'radar',
      data: [{
        value: values,
        name: t('dataCharts.featureUsage'),
        areaStyle: { opacity: 0.2, color: '#A855F7' },
        lineStyle: { color: '#A855F7', width: 2 },
        itemStyle: { color: '#A855F7' },
      }],
    }],
  }
})

// ── Member distribution bar chart ──
const memberBarOption = computed(() => {
  const orgNames = organizations.value.map(o => o.name.length > 8 ? o.name.slice(0, 8) + '…' : o.name)
  const memberCounts = organizations.value.map(o => o.member_count || 0)

  if (!orgNames.length) return null

  return {
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    grid: { left: '3%', right: '4%', bottom: '8%', top: '10%', containLabel: true },
    xAxis: {
      type: 'category',
      data: orgNames,
      axisLabel: { rotate: orgNames.length > 5 ? 30 : 0, fontSize: 11, color: T.value.textSec },
      axisLine: { lineStyle: { color: T.value.border } },
    },
    yAxis: {
      type: 'value',
      minInterval: 1,
      axisLabel: { color: T.value.textSec },
      splitLine: { lineStyle: { color: T.value.border, type: 'dashed' } },
    },
    series: [{
      type: 'bar',
      data: memberCounts,
      barMaxWidth: 40,
      itemStyle: {
        borderRadius: [4, 4, 0, 0],
        color: {
          type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
          colorStops: [
            { offset: 0, color: '#F59E0B' },
            { offset: 1, color: '#F59E0B44' },
          ],
        },
      },
    }],
  }
})

// ── Radar chart ──
const radarOption = computed(() => {
  if (organizations.value.length === 0) return null

  const topOrgs = organizations.value.slice(0, 5)
  const maxMembers = Math.max(...topOrgs.map(o => o.member_count || 0), 1)
  const maxShares = Math.max(...topOrgs.map(o => o.share_count || 0), 1)
  const maxAgentShares = Math.max(...topOrgs.map(o => o.agent_share_count || 0), 1)
  const maxPending = Math.max(...topOrgs.map(o => o.pending_join_request_count || 0), 1)

  const indicator = [
    { name: t('dataCharts.members'), max: 100 },
    { name: t('dataCharts.kbShares'), max: 100 },
    { name: t('dataCharts.agentShares'), max: 100 },
    { name: t('dataCharts.pendingRequests'), max: 100 },
  ]

  const seriesData = topOrgs.map((org, i) => ({
    name: org.name.length > 10 ? org.name.slice(0, 10) + '…' : org.name,
    value: [
      Math.round(((org.member_count || 0) / maxMembers) * 100),
      Math.round(((org.share_count || 0) / maxShares) * 100),
      Math.round(((org.agent_share_count || 0) / maxAgentShares) * 100),
      Math.round(((org.pending_join_request_count || 0) / maxPending) * 100),
    ],
    areaStyle: { opacity: 0.15 },
    lineStyle: { width: 2 },
    itemStyle: { color: COLORS[i % COLORS.length] },
  }))

  return {
    tooltip: { trigger: 'item' },
    legend: {
      bottom: '0%',
      textStyle: { color: T.value.textSec, fontSize: 10 },
    },
    radar: {
      indicator,
      center: ['50%', '48%'],
      radius: '60%',
      axisName: { color: T.value.textSec, fontSize: 11 },
      splitArea: { areaStyle: { color: ['transparent'] } },
      splitLine: { lineStyle: { color: T.value.border } },
      axisLine: { lineStyle: { color: T.value.border } },
    },
    series: [{
      type: 'radar',
      data: seriesData,
    }],
  }
})

// ── Share heatmap ──
const shareOption = computed(() => {
  if (!organizations.value.length) return null

  const orgNames = organizations.value.map(o => o.name.length > 10 ? o.name.slice(0, 10) + '…' : o.name)
  const metrics = [t('dataCharts.kbShares'), t('dataCharts.agentShares'), t('dataCharts.pendingRequests')]
  const data: [number, number, number][] = []

  organizations.value.forEach((org, oi) => {
    const values = [
      org.share_count || 0,
      org.agent_share_count || 0,
      org.pending_join_request_count || 0,
    ]
    values.forEach((v, mi) => {
      data.push([oi, mi, v])
    })
  })

  const maxVal = Math.max(...data.map(d => d[2]), 1)

  return {
    tooltip: {
      formatter: (params: any) => {
        const [oi, mi, val] = params.data
        return `${orgNames[oi]} · ${metrics[mi]}: ${val}`
      },
    },
    grid: { left: '15%', right: '12%', bottom: '15%', top: '8%' },
    xAxis: {
      type: 'category',
      data: orgNames,
      axisLabel: { rotate: 30, fontSize: 10, color: T.value.textSec },
      axisLine: { lineStyle: { color: T.value.border } },
    },
    yAxis: {
      type: 'category',
      data: metrics,
      axisLabel: { fontSize: 10, color: T.value.textSec },
      axisLine: { lineStyle: { color: T.value.border } },
    },
    visualMap: {
      min: 0,
      max: maxVal,
      calculable: true,
      orient: 'horizontal',
      left: 'center',
      bottom: '0%',
      inRange: {
        color: T.value.heatmapRange,
      },
      textStyle: { color: T.value.textSec, fontSize: 10 },
    },
    series: [{
      type: 'heatmap',
      data,
      label: { show: true, fontSize: 11, color: T.value.text },
      emphasis: {
        itemStyle: { shadowBlur: 10, shadowColor: 'rgba(0, 0, 0, 0.5)' },
      },
    }],
  }
})
</script>

<style scoped lang="less">
.data-charts-page {
  height: 100%;
  box-sizing: border-box;
  overflow: auto;
  padding: 24px 28px 32px;
}

.charts-header {
  margin-bottom: 24px;

  .charts-title {
    font-size: 22px;
    font-weight: 600;
    color: var(--td-text-color-primary);
    margin: 0 0 4px;
  }

  .charts-desc {
    font-size: 13px;
    color: var(--td-text-color-secondary);
    margin: 0;
  }
}

// ── Stat cards ──
.stats-cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 16px;
  margin-bottom: 24px;
}

.stat-card {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 18px 20px;
  border-radius: 10px;
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-border-level-1-color);
  transition: box-shadow 0.2s;

  &:hover {
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.08);
  }
}

.stat-icon {
  width: 48px;
  height: 48px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.stat-emoji {
  font-size: 22px;
}

.stat-info {
  display: flex;
  flex-direction: column;
}

.stat-value {
  font-size: 26px;
  font-weight: 700;
  color: var(--td-text-color-primary);
  line-height: 1.2;
}

.stat-label {
  font-size: 12px;
  color: var(--td-text-color-secondary);
  margin-top: 2px;
}

// ── Chart grid ──
.charts-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 16px;
}

.chart-card {
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-border-level-1-color);
  border-radius: 10px;
  padding: 16px 20px;
  transition: box-shadow 0.2s;

  &:hover {
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.06);
  }

  &--wide {
    grid-column: 1 / -1;
  }
}

.chart-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;

  h3 {
    font-size: 15px;
    font-weight: 600;
    color: var(--td-text-color-primary);
    margin: 0;
  }
}

.chart-container {
  width: 100%;
  height: 300px;

  &--tall {
    height: 400px;
  }

  :deep(.v-chart) {
    width: 100% !important;
    height: 100% !important;
  }
}

// ── Dark mode ──
html[theme-mode="dark"] {
  .stat-card {
    background: var(--td-bg-color-container);
    border-color: var(--td-border-level-2-color);
  }

  .chart-card {
    background: var(--td-bg-color-container);
    border-color: var(--td-border-level-2-color);
  }
}

// ── Responsive ──
@media (max-width: 900px) {
  .charts-grid {
    grid-template-columns: 1fr;
  }

  .chart-card--wide {
    grid-column: 1;
  }

  .stats-cards {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 600px) {
  .stats-cards {
    grid-template-columns: 1fr;
  }

  .data-charts-page {
    padding: 16px;
  }
}
</style>
