<template>
  <div class="card overflow-hidden p-4">
    <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">
      {{ t('admin.dashboard.tokenUsageTrend') }}
    </h3>
    <div class="h-64 lg:h-72">
      <div v-if="loading" class="flex h-full items-center justify-center">
        <LoadingSpinner />
      </div>
      <Line v-else-if="trendData.length > 0 && chartData" :data="chartData" :options="lineOptions" />
      <div
        v-else
        class="flex h-full items-center justify-center text-sm text-gray-500 dark:text-gray-400"
      >
        {{ t('admin.dashboard.noDataAvailable') }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'
import { Line } from 'vue-chartjs'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import type { TrendDataPoint } from '@/types'

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler
)

const { t } = useI18n()

const props = withDefaults(
  defineProps<{
    trendData: TrendDataPoint[]
    loading?: boolean
    granularity?: 'day' | 'hour'
  }>(),
  {
    granularity: 'day'
  }
)

const isDarkMode = computed(() => {
  return document.documentElement.classList.contains('dark')
})

const chartColors = computed(() => ({
  text: isDarkMode.value ? '#e5e7eb' : '#374151',
  grid: isDarkMode.value ? '#374151' : '#e5e7eb',
  input: '#3b82f6',
  output: '#10b981',
  cache: '#f59e0b'
}))

const toNonNegativeToken = (value: number): number => {
  if (!Number.isFinite(value)) return 0
  return Math.max(0, value)
}

const formatHourMinuteLabel = (label: string): string => {
  const trimmed = label.trim()
  if (!trimmed) return ''
  const splitter = trimmed.includes(' ') ? ' ' : trimmed.includes('T') ? 'T' : ''
  const timePart = splitter ? trimmed.split(splitter).pop() || '' : trimmed
  const cleaned = timePart.replace(/Z|[+-]\d{2}:?\d{2}$/, '')
  return cleaned.slice(0, 5)
}

const formatXAxisLabel = (label: string): string => {
  if (props.granularity !== 'hour') return label
  return formatHourMinuteLabel(label)
}

const shouldUseKUnitByDefault = computed(() => {
  if (!props.trendData?.length) return true
  return props.trendData.every(
    (d) =>
      toNonNegativeToken(d.input_tokens) === 0 &&
      toNonNegativeToken(d.output_tokens) === 0 &&
      toNonNegativeToken(d.cache_tokens) === 0
  )
})

const chartData = computed(() => {
  if (!props.trendData?.length) return null

  return {
    labels: props.trendData.map((d) => d.date),
    datasets: [
      {
        label: 'Input',
        data: props.trendData.map((d) => toNonNegativeToken(d.input_tokens)),
        borderColor: chartColors.value.input,
        backgroundColor: `${chartColors.value.input}20`,
        fill: true,
        tension: 0.3
      },
      {
        label: 'Output',
        data: props.trendData.map((d) => toNonNegativeToken(d.output_tokens)),
        borderColor: chartColors.value.output,
        backgroundColor: `${chartColors.value.output}20`,
        fill: true,
        tension: 0.3
      },
      {
        label: 'Cache',
        data: props.trendData.map((d) => toNonNegativeToken(d.cache_tokens)),
        borderColor: chartColors.value.cache,
        backgroundColor: `${chartColors.value.cache}20`,
        fill: true,
        tension: 0.3
      }
    ]
  }
})

const lineOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  elements: {
    point: {
      radius: 0,
      hoverRadius: 4,
      hitRadius: 6
    }
  },
  interaction: {
    intersect: false,
    mode: 'index' as const
  },
  plugins: {
    legend: {
      position: 'top' as const,
      labels: {
        color: chartColors.value.text,
        usePointStyle: true,
        pointStyle: 'circle',
        padding: 15,
        font: {
          size: 11
        }
      }
    },
    tooltip: {
      callbacks: {
        label: (context: any) => {
          return `${context.dataset.label}: ${formatTokens(
            context.raw,
            shouldUseKUnitByDefault.value ? 'K' : undefined
          )}`
        },
        footer: (tooltipItems: any) => {
          const dataIndex = tooltipItems[0]?.dataIndex
          if (dataIndex !== undefined && props.trendData[dataIndex]) {
            const data = props.trendData[dataIndex]
            return `Actual: $${formatCost(data.actual_cost)} | Standard: $${formatCost(data.cost)}`
          }
          return ''
        }
      }
    }
  },
  scales: {
    x: {
      grid: {
        color: chartColors.value.grid
      },
      ticks: {
        color: chartColors.value.text,
        font: {
          size: 10
        },
        callback: function (value: string | number) {
          const label =
            typeof value === 'string'
              ? value
              : (this as { getLabelForValue?: (v: string | number) => string }).getLabelForValue?.(
                  value
                ) ?? String(value)
          return formatXAxisLabel(label)
        }
      }
    },
    y: {
      beginAtZero: true,
      ...(shouldUseKUnitByDefault.value ? { suggestedMax: 1000 } : {}),
      grid: {
        color: chartColors.value.grid
      },
      ticks: {
        color: chartColors.value.text,
        font: {
          size: 10
        },
        callback: (value: string | number) =>
          formatTokens(Number(value), shouldUseKUnitByDefault.value ? 'K' : undefined)
      }
    }
  }
}))

const formatTokens = (value: number, forceUnit?: 'K'): string => {
  if (!Number.isFinite(value)) return forceUnit === 'K' ? '0K' : '0'
  value = Math.max(0, value)

  if (forceUnit === 'K') {
    const k = value / 1000
    const text = k.toFixed(2).replace(/\.00$/, '').replace(/(\.\d)0$/, '$1')
    return `${text}K`
  }

  if (value >= 1_000_000_000) {
    return `${(value / 1_000_000_000).toFixed(2)}B`
  } else if (value >= 1_000_000) {
    return `${(value / 1_000_000).toFixed(2)}M`
  } else if (value >= 1_000) {
    return `${(value / 1_000).toFixed(2)}K`
  }
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
