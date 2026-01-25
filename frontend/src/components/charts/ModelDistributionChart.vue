<template>
  <template v-if="split">
    <!-- Model Distribution Chart -->
    <div class="card overflow-hidden p-4">
      <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">
        {{ t('admin.dashboard.modelDistribution') }}
      </h3>
      <div v-if="loading" class="flex h-48 items-center justify-center">
        <LoadingSpinner />
      </div>
      <div v-else-if="modelChartData" class="flex flex-col items-center gap-4">
        <div class="h-48 w-48 shrink-0">
          <Doughnut ref="modelChartRef" :data="modelChartData" :options="doughnutOptions" />
        </div>
        <div class="w-full">
          <ul class="flex flex-wrap justify-center gap-x-4 gap-y-2 text-xs">
            <li
              v-for="(item, index) in modelLegendItems"
              :key="item.label"
              class="flex min-w-0 cursor-pointer select-none items-center gap-2 rounded-lg px-2 py-1 text-gray-700 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-dark-800/50"
              :class="!isModelDataVisible(index) ? 'opacity-50 line-through' : ''"
              :title="item.label"
              @click="toggleModelDataVisibility(index)"
            >
              <span class="h-2.5 w-2.5 shrink-0 rounded-sm" :style="{ backgroundColor: item.color }" />
              <span class="max-w-[140px] truncate">{{ item.label }}</span>
            </li>
          </ul>
        </div>
      </div>
      <div
        v-else
        class="flex h-48 items-center justify-center text-sm text-gray-500 dark:text-gray-400"
      >
        {{ t('admin.dashboard.noDataAvailable') }}
      </div>
    </div>

    <!-- Model Distribution Table -->
    <div class="card overflow-hidden p-4">
      <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">
        {{ t('admin.dashboard.modelDistribution') }} · {{ t('admin.dashboard.model') }}
      </h3>
      <div v-if="loading" class="flex h-48 items-center justify-center">
        <LoadingSpinner />
      </div>
      <div v-else-if="modelStats.length > 0" class="overflow-y-auto">
        <table class="w-full text-xs">
          <thead>
            <tr class="text-gray-500 dark:text-gray-400">
              <th class="pb-2 text-left">{{ t('admin.dashboard.model') }}</th>
              <th class="pb-2 text-right">{{ t('admin.dashboard.requests') }}</th>
              <th class="pb-2 text-right">{{ t('admin.dashboard.tokens') }}</th>
              <th class="pb-2 text-right">{{ t('admin.dashboard.actual') }}</th>
              <th class="pb-2 text-right">{{ t('admin.dashboard.standard') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="model in modelStats"
              :key="model.model"
              class="border-t border-gray-100 dark:border-gray-700"
            >
              <td
                class="max-w-[100px] truncate py-1.5 font-medium text-gray-900 dark:text-white"
                :title="model.model"
              >
                {{ model.model }}
              </td>
              <td class="py-1.5 text-right text-gray-600 dark:text-gray-400">
                {{ formatNumber(model.requests) }}
              </td>
              <td class="py-1.5 text-right text-gray-600 dark:text-gray-400">
                {{ formatTokens(model.total_tokens) }}
              </td>
              <td class="py-1.5 text-right text-green-600 dark:text-green-400">
                ${{ formatCost(model.actual_cost) }}
              </td>
              <td class="py-1.5 text-right text-gray-400 dark:text-gray-500">
                ${{ formatCost(model.cost) }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div
        v-else
        class="flex h-48 items-center justify-center text-sm text-gray-500 dark:text-gray-400"
      >
        {{ t('admin.dashboard.noDataAvailable') }}
      </div>
    </div>
  </template>

  <div v-else class="card p-4">
    <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">
      {{ t('admin.dashboard.modelDistribution') }}
    </h3>
    <div v-if="loading" class="flex h-48 items-center justify-center">
      <LoadingSpinner />
    </div>
    <div v-else-if="modelStats.length > 0 && modelChartData" class="flex items-center gap-6">
      <div class="h-48 w-48">
        <Doughnut :data="modelChartData" :options="doughnutOptions" />
      </div>
      <div class="flex-1 overflow-y-auto">
        <table class="w-full text-xs">
          <thead>
            <tr class="text-gray-500 dark:text-gray-400">
              <th class="pb-2 text-left">{{ t('admin.dashboard.model') }}</th>
              <th class="pb-2 text-right">{{ t('admin.dashboard.requests') }}</th>
              <th class="pb-2 text-right">{{ t('admin.dashboard.tokens') }}</th>
              <th class="pb-2 text-right">{{ t('admin.dashboard.actual') }}</th>
              <th class="pb-2 text-right">{{ t('admin.dashboard.standard') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="model in modelStats"
              :key="model.model"
              class="border-t border-gray-100 dark:border-gray-700"
            >
              <td
                class="max-w-[100px] truncate py-1.5 font-medium text-gray-900 dark:text-white"
                :title="model.model"
              >
                {{ model.model }}
              </td>
              <td class="py-1.5 text-right text-gray-600 dark:text-gray-400">
                {{ formatNumber(model.requests) }}
              </td>
              <td class="py-1.5 text-right text-gray-600 dark:text-gray-400">
                {{ formatTokens(model.total_tokens) }}
              </td>
              <td class="py-1.5 text-right text-green-600 dark:text-green-400">
                ${{ formatCost(model.actual_cost) }}
              </td>
              <td class="py-1.5 text-right text-gray-400 dark:text-gray-500">
                ${{ formatCost(model.cost) }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
    <div
      v-else
      class="flex h-48 items-center justify-center text-sm text-gray-500 dark:text-gray-400"
    >
      {{ t('admin.dashboard.noDataAvailable') }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Chart as ChartJS, ArcElement, Tooltip, Legend } from 'chart.js'
import { Doughnut } from 'vue-chartjs'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import type { ModelStat } from '@/types'

ChartJS.register(ArcElement, Tooltip, Legend)

const { t } = useI18n()

const props = defineProps<{
  modelStats: ModelStat[]
  loading?: boolean
  split?: boolean
}>()

type ChartComponentRef = { chart?: ChartJS }
const modelChartRef = ref<ChartComponentRef | null>(null)

const modelChartData = computed(() => {
  if (!props.modelStats?.length) return null

  const colors = [
    '#3b82f6',
    '#10b981',
    '#f59e0b',
    '#ef4444',
    '#8b5cf6',
    '#ec4899',
    '#14b8a6',
    '#f97316',
    '#6366f1',
    '#84cc16'
  ]

  const backgroundColor = props.modelStats.map((_, index) => colors[index % colors.length])
  return {
    labels: props.modelStats.map((m) => m.model),
    datasets: [
      {
        data: props.modelStats.map((m) => m.total_tokens),
        backgroundColor,
        borderWidth: 0
      }
    ]
  }
})

const modelLegendItems = computed(() => {
  if (!modelChartData.value) return []
  const dataset = modelChartData.value.datasets?.[0]
  const backgroundColor = (dataset?.backgroundColor || []) as string[]
  const labels = (modelChartData.value.labels || []) as string[]
  return labels.map((label, index) => ({
    label,
    color: backgroundColor[index] || '#9ca3af'
  }))
})

const modelLegendVersion = ref(0)

const isModelDataVisible = (index: number): boolean => {
  modelLegendVersion.value
  const chart = modelChartRef.value?.chart
  if (!chart) return true
  return chart.getDataVisibility(index)
}

const toggleModelDataVisibility = (index: number) => {
  const chart = modelChartRef.value?.chart
  if (!chart) return
  chart.toggleDataVisibility(index)
  chart.update()
  modelLegendVersion.value++
}

const doughnutOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: {
      display: false
    },
    tooltip: {
      callbacks: {
        label: (context: any) => {
          const value = context.raw as number
          const total = context.dataset.data.reduce((a: number, b: number) => a + b, 0)
          const percentage = ((value / total) * 100).toFixed(1)
          return `${context.label}: ${formatTokens(value)} (${percentage}%)`
        }
      }
    }
  }
}))

const formatTokens = (value: number): string => {
  if (value >= 1_000_000_000) {
    return `${(value / 1_000_000_000).toFixed(2)}B`
  } else if (value >= 1_000_000) {
    return `${(value / 1_000_000).toFixed(2)}M`
  } else if (value >= 1_000) {
    return `${(value / 1_000).toFixed(2)}K`
  }
  return value.toLocaleString()
}

const formatNumber = (value: number): string => {
  return value.toLocaleString()
}

const formatCost = (value: number): string => {
  if (value >= 1000) {
    return (value / 1000).toFixed(2) + 'K'
  } else if (value >= 1) {
    return value.toFixed(2)
  } else if (value >= 0.01) {
    return value.toFixed(3)
  }
  return value.toFixed(4)
}
</script>
