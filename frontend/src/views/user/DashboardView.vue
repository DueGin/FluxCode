<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- Loading State -->
      <div v-if="loading" class="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>

      <template v-else-if="stats">
        <!-- Row 1: Core Stats -->
        <div class="grid grid-cols-4 gap-4">

          <!-- Today Requests -->
          <div class="card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-green-100 p-2 dark:bg-green-900/30">
                <svg
                  class="h-5 w-5 text-green-600 dark:text-green-400"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 013 19.875v-6.75zM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V8.625zM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V4.125z"
                  />
                </svg>
              </div>
              <div>
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                  {{ t('dashboard.todayRequests') }}
                </p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">
                  {{ stats.today_requests }}
                </p>
                <p class="text-xs text-gray-500 dark:text-gray-400">
                  {{ t('common.total') }}: {{ formatNumber(stats.total_requests) }}
                </p>
              </div>
            </div>
          </div>

          <!-- Today Cost -->
          <div class="card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-purple-100 p-2 dark:bg-purple-900/30">
                <svg
                  class="h-5 w-5 text-purple-600 dark:text-purple-400"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M12 6v12m-3-2.818l.879.659c1.171.879 3.07.879 4.242 0 1.172-.879 1.172-2.303 0-3.182C13.536 12.219 12.768 12 12 12c-.725 0-1.45-.22-2.003-.659-1.106-.879-1.106-2.303 0-3.182s2.9-.879 4.006 0l.415.33M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                  />
                </svg>
              </div>
              <div>
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                  {{ t('dashboard.todayCost') }}
                </p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">
                  <span class="text-purple-600 dark:text-purple-400" :title="t('dashboard.actual')"
                    >${{ formatCost(stats.today_actual_cost) }}</span
                  >
                  <span
                    class="text-sm font-normal text-gray-400 dark:text-gray-500"
                    :title="t('dashboard.dailyLimit')"
                  >
                    / ${{ formatCost(stats.today_cost) }}</span
                  >
                </p>
                <p class="text-xs">
                  <span class="text-gray-500 dark:text-gray-400">{{ t('common.total') }}: </span>
                  <span class="text-purple-600 dark:text-purple-400" :title="t('dashboard.actual')"
                    >${{ formatCost(stats.total_actual_cost) }}</span
                  >
                  <!-- <span class="text-gray-400 dark:text-gray-500" :title="t('dashboard.standard')">
                    / ${{ formatCost(stats.total_cost) }}</span
                  > -->
                </p>
              </div>
            </div>
          </div>

          <!-- Today Tokens -->
          <div class="card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-amber-100 p-2 dark:bg-amber-900/30">
                <svg
                  class="h-5 w-5 text-amber-600 dark:text-amber-400"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="m21 7.5-9-5.25L3 7.5m18 0-9 5.25m9-5.25v9l-9 5.25M3 7.5l9 5.25M3 7.5v9l9 5.25m0-9v9"
                  />
                </svg>
              </div>
              <div>
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                  {{ t('dashboard.todayTokens') }}
                </p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">
                  {{ formatTokens(stats.today_tokens) }}
                </p>
                <p class="text-xs text-gray-500 dark:text-gray-400">
                  {{ t('dashboard.input') }}: {{ formatTokens(stats.today_input_tokens) }} /
                  {{ t('dashboard.output') }}: {{ formatTokens(stats.today_output_tokens) }}
                </p>
              </div>
            </div>
          </div>

          <!-- Total Tokens -->
          <div class="card p-4">
            <div class="flex items-center gap-3">
              <div class="rounded-lg bg-indigo-100 p-2 dark:bg-indigo-900/30">
                <svg
                  class="h-5 w-5 text-indigo-600 dark:text-indigo-400"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M20.25 6.375c0 2.278-3.694 4.125-8.25 4.125S3.75 8.653 3.75 6.375m16.5 0c0-2.278-3.694-4.125-8.25-4.125S3.75 4.097 3.75 6.375m16.5 0v11.25c0 2.278-3.694 4.125-8.25 4.125s-8.25-1.847-8.25-4.125V6.375m16.5 0v3.75m-16.5-3.75v3.75m16.5 0v3.75C20.25 16.153 16.556 18 12 18s-8.25-1.847-8.25-4.125v-3.75m16.5 0c0 2.278-3.694 4.125-8.25 4.125s-8.25-1.847-8.25-4.125"
                  />
                </svg>
              </div>
              <div>
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                  {{ t('dashboard.totalTokens') }}
                </p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">
                  {{ formatTokens(stats.total_tokens) }}
                </p>
                <p class="text-xs text-gray-500 dark:text-gray-400">
                  {{ t('dashboard.input') }}: {{ formatTokens(stats.total_input_tokens) }} /
                  {{ t('dashboard.output') }}: {{ formatTokens(stats.total_output_tokens) }}
                </p>
              </div>
            </div>
          </div>
        </div>

        <!-- Charts Section -->
        <div class="space-y-6">
          <!-- Date Range Filter -->
          <div class="card p-4">
            <div class="flex flex-wrap items-center gap-4">
              <div class="flex items-center gap-2">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300"
                  >{{ t('dashboard.timeRange') }}:</span
                >
                <div class="inline-flex rounded-lg bg-gray-100 p-1 dark:bg-dark-700">
                  <button
                    v-for="item in timeRangeTabs"
                    :key="item.value"
                    type="button"
                    @click="selectTimeRange(item.value)"
                    class="rounded-md px-3 py-1.5 text-xs font-medium transition-colors duration-150"
                    :class="
                      isTimeRangeActive(item.value)
                        ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-800 dark:text-white'
                        : 'text-gray-600 hover:bg-gray-200/70 dark:text-gray-300 dark:hover:bg-dark-600/50'
                    "
                  >
                    {{ item.label }}
                  </button>
                </div>
              </div>
            </div>
	          </div>
	
	          <!-- Charts Grid -->
	          <div class="relative">
	            <div
	              v-if="loadingCharts"
	              class="absolute inset-0 z-10 flex items-center justify-center bg-white/50 backdrop-blur-sm dark:bg-dark-800/50"
	            >
	              <LoadingSpinner size="md" />
	            </div>
	
	            <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
	              <!-- Model Distribution Chart -->
	              <div class="card overflow-hidden p-4">
	                <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">
	                  {{ t('dashboard.modelDistribution') }}
	                </h3>
		                <div v-if="modelChartData" class="flex flex-col items-center gap-4">
		                  <div class="h-48 w-48 shrink-0">
		                    <Doughnut
		                      ref="modelChartRef"
		                      :data="modelChartData"
		                      :options="doughnutOptions"
		                    />
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
	                  {{ t('dashboard.noDataAvailable') }}
	                </div>
	              </div>
	
	              <!-- Model Distribution Table -->
	              <div class="card overflow-hidden p-4">
	                <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">
	                  {{ t('dashboard.modelDistribution') }} · {{ t('dashboard.model') }}
	                </h3>
                <div v-if="modelStats.length > 0" class="overflow-y-auto">
	                  <table class="w-full text-xs">
	                    <thead>
	                      <tr class="text-gray-500 dark:text-gray-400">
	                        <th class="pb-2 text-left">{{ t('dashboard.model') }}</th>
	                        <th class="pb-2 text-right">{{ t('dashboard.requests') }}</th>
	                        <th class="pb-2 text-right">{{ t('dashboard.tokens') }}</th>
	                        <th class="pb-2 text-right">{{ t('dashboard.actual') }}</th>
	                        <th class="pb-2 text-right">{{ t('dashboard.standard') }}</th>
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
	                  {{ t('dashboard.noDataAvailable') }}
	                </div>
	              </div>
	
	              <!-- Token Usage Trend Chart -->
	              <div class="card overflow-hidden p-4 lg:col-span-2">
	                <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">
	                  {{ t('dashboard.tokenUsageTrend') }}
	                </h3>
	                <div class="h-64 lg:h-72">
	                  <Line
	                    v-if="trendChartData"
	                    ref="trendChartRef"
	                    :data="trendChartData"
	                    :options="lineOptions"
	                  />
	                  <div
	                    v-else
	                    class="flex h-full items-center justify-center text-sm text-gray-500 dark:text-gray-400"
	                  >
	                    {{ t('dashboard.noDataAvailable') }}
	                  </div>
	                </div>
	              </div>
	            </div>
	          </div>
	        </div>
	
	        <!-- Main Content Grid -->
	        <div class="grid grid-cols-1 gap-6 lg:grid-cols-3">
          <!-- Recent Usage - Takes 2 columns -->
          <div class="lg:col-span-2">
            <div class="card">
              <div
                class="flex items-center justify-between border-b border-gray-100 px-6 py-4 dark:border-dark-700"
              >
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                  {{ t('dashboard.recentUsage') }}
                </h2>
                <span class="badge badge-gray">{{ t('dashboard.last7Days') }}</span>
              </div>
              <div class="p-6">
                <div v-if="loadingUsage" class="flex items-center justify-center py-12">
                  <LoadingSpinner size="lg" />
                </div>
                <div v-else-if="recentUsage.length === 0" class="py-8">
                  <EmptyState
                    :title="t('dashboard.noUsageRecords')"
                    :description="t('dashboard.startUsingApi')"
                  />
                </div>
                <div v-else class="space-y-3">
                  <div
                    v-for="log in recentUsage"
                    :key="log.id"
                    class="flex items-center justify-between rounded-xl bg-gray-50 p-4 transition-colors hover:bg-gray-100 dark:bg-dark-800/50 dark:hover:bg-dark-800"
                  >
                    <div class="flex items-center gap-4">
                      <div
                        class="flex h-10 w-10 items-center justify-center rounded-xl bg-primary-100 dark:bg-primary-900/30"
                      >
                        <svg
                          class="h-5 w-5 text-primary-600 dark:text-primary-400"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                          stroke-width="1.5"
                        >
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            d="M9.75 3.104v5.714a2.25 2.25 0 01-.659 1.591L5 14.5M9.75 3.104c-.251.023-.501.05-.75.082m.75-.082a24.301 24.301 0 014.5 0m0 0v5.714c0 .597.237 1.17.659 1.591L19.8 15.3M14.25 3.104c.251.023.501.05.75.082M19.8 15.3l-1.57.393A9.065 9.065 0 0112 15a9.065 9.065 0 00-6.23.693L5 14.5m14.8.8l1.402 1.402c1.232 1.232.65 3.318-1.067 3.611A48.309 48.309 0 0112 21c-2.773 0-5.491-.235-8.135-.687-1.718-.293-2.3-2.379-1.067-3.61L5 14.5"
                          />
                        </svg>
                      </div>
                      <div>
                        <p class="text-sm font-medium text-gray-900 dark:text-white">
                          {{ log.model }}
                        </p>
                        <p class="text-xs text-gray-500 dark:text-dark-400">
                          {{ formatDateTime(log.created_at) }}
                        </p>
                      </div>
                    </div>
                    <div class="text-right">
                      <p class="text-sm font-semibold">
                        <span class="text-green-600 dark:text-green-400" :title="t('dashboard.actual')"
                          >${{ formatCost(log.actual_cost) }}</span
                        >
                        <span class="font-normal text-gray-400 dark:text-gray-500" :title="t('dashboard.standard')">
                          / ${{ formatCost(log.total_cost) }}</span
                        >
                      </p>
                      <p class="text-xs text-gray-500 dark:text-dark-400">
                        {{ (log.input_tokens + log.output_tokens).toLocaleString() }} tokens
                      </p>
                    </div>
                  </div>

                  <router-link
                    to="/usage"
                    class="flex items-center justify-center gap-2 py-3 text-sm font-medium text-primary-600 transition-colors hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
                  >
                    {{ t('dashboard.viewAllUsage') }}
                    <svg
                      class="h-4 w-4"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                      stroke-width="1.5"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        d="M13.5 4.5L21 12m0 0l-7.5 7.5M21 12H3"
                      />
                    </svg>
                  </router-link>
                </div>
              </div>
            </div>
          </div>

          <!-- Quick Actions - Takes 1 column -->
          <div class="space-y-6 lg:col-span-1">
            <div class="card">
              <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                  {{ t('dashboard.quickActions') }}
                </h2>
              </div>
              <div class="space-y-3 p-4">
                <button
                  @click="navigateTo('/keys')"
                  class="group flex w-full items-center gap-4 rounded-xl bg-gray-50 p-4 text-left transition-all duration-200 hover:bg-gray-100 dark:bg-dark-800/50 dark:hover:bg-dark-800"
                >
                  <div
                    class="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-xl bg-primary-100 transition-transform group-hover:scale-105 dark:bg-primary-900/30"
                  >
                    <svg
                      class="h-6 w-6 text-primary-600 dark:text-primary-400"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                      stroke-width="1.5"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        d="M15.75 5.25a3 3 0 013 3m3 0a6 6 0 01-7.029 5.912c-.563-.097-1.159.026-1.563.43L10.5 17.25H8.25v2.25H6v2.25H2.25v-2.818c0-.597.237-1.17.659-1.591l6.499-6.499c.404-.404.527-1 .43-1.563A6 6 0 1121.75 8.25z"
                      />
                    </svg>
                  </div>
                  <div class="min-w-0 flex-1">
                    <p class="text-sm font-medium text-gray-900 dark:text-white">
                      {{ t('dashboard.createApiKey') }}
                    </p>
                    <p class="text-xs text-gray-500 dark:text-dark-400">
                      {{ t('dashboard.generateNewKey') }}
                    </p>
                  </div>
                  <svg
                    class="h-5 w-5 text-gray-400 transition-colors group-hover:text-primary-500 dark:text-dark-500"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    stroke-width="1.5"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M8.25 4.5l7.5 7.5-7.5 7.5"
                    />
                  </svg>
                </button>

                <button
                  @click="navigateTo('/usage')"
                  class="group flex w-full items-center gap-4 rounded-xl bg-gray-50 p-4 text-left transition-all duration-200 hover:bg-gray-100 dark:bg-dark-800/50 dark:hover:bg-dark-800"
                >
                  <div
                    class="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-xl bg-emerald-100 transition-transform group-hover:scale-105 dark:bg-emerald-900/30"
                  >
                    <svg
                      class="h-6 w-6 text-emerald-600 dark:text-emerald-400"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                      stroke-width="1.5"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        d="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 013 19.875v-6.75zM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V8.625zM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V4.125z"
                      />
                    </svg>
                  </div>
                  <div class="min-w-0 flex-1">
                    <p class="text-sm font-medium text-gray-900 dark:text-white">
                      {{ t('dashboard.viewUsage') }}
                    </p>
                    <p class="text-xs text-gray-500 dark:text-dark-400">
                      {{ t('dashboard.checkDetailedLogs') }}
                    </p>
                  </div>
                  <svg
                    class="h-5 w-5 text-gray-400 transition-colors group-hover:text-emerald-500 dark:text-dark-500"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    stroke-width="1.5"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M8.25 4.5l7.5 7.5-7.5 7.5"
                    />
                  </svg>
                </button>
              </div>
            </div>

            <!-- After-sale Contact -->
            <div class="card">
              <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                  {{ t('dashboard.afterSaleContact') }}
                </h2>
              </div>
              <div class="p-4">
                <div v-if="afterSaleContactItems.length > 0" class="grid grid-cols-1 gap-3">
                  <div
                    v-for="(item, index) in afterSaleContactItems"
                    :key="`${item.k}-${index}`"
                    class="mx-auto grid w-[92%] max-w-[360px] grid-cols-[72px,1fr] items-center gap-3 rounded-xl bg-gray-50 px-4 py-2.5 dark:bg-dark-800/50"
                  >
                    <span
                      class="truncate text-base font-semibold text-gray-700 dark:text-gray-300"
                      :title="item.k"
                      >{{ item.k }}</span
                    >
                    <span class="min-w-0 truncate text-sm text-gray-600 dark:text-dark-300" :title="item.v">{{
                      item.v
                    }}</span>
                  </div>
                </div>
                <p v-else class="text-sm text-gray-500 dark:text-dark-400">
                  {{ t('dashboard.afterSaleContactEmpty') }}
                </p>
              </div>
            </div>
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { useSubscriptionStore } from '@/stores/subscriptions'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
import { usageAPI, type TrendParams as UserTrendParams, type UserDashboardStats } from '@/api/usage'
import type { UsageLog, TrendDataPoint, ModelStat } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import EmptyState from '@/components/common/EmptyState.vue'

import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  ArcElement,
  Title,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'
import { Line, Doughnut } from 'vue-chartjs'

// Register Chart.js components
ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  ArcElement,
  Title,
  Tooltip,
  Legend,
  Filler
)

const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()
const subscriptionStore = useSubscriptionStore()

const user = computed(() => authStore.user)
const afterSaleContactItems = computed(() => appStore.afterSaleContact || [])
const stats = ref<UserDashboardStats | null>(null)
const loading = ref(false)
const loadingUsage = ref(false)
const loadingCharts = ref(false)

type ChartComponentRef = { chart?: ChartJS }

// Chart data
const trendData = ref<TrendDataPoint[]>([])
const modelStats = ref<ModelStat[]>([])
const modelChartRef = ref<ChartComponentRef | null>(null)
const trendChartRef = ref<ChartComponentRef | null>(null)

// Recent usage
const recentUsage = ref<UsageLog[]>([])

// Helper function to format date in local timezone
const formatLocalDate = (date: Date): string => {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

// Initialize date range immediately (not in onMounted)
const now = new Date()
const weekAgo = new Date(now)
weekAgo.setDate(weekAgo.getDate() - 6)

// Date range
const startDate = ref(formatLocalDate(weekAgo))
const endDate = ref(formatLocalDate(now))

type TimeRangeTab = '24h' | '7d' | '14d' | '30d'

const timeRange = ref<TimeRangeTab>('24h')

const timeRangeTabs = computed(() => [
  { value: '24h' as const, label: t('dashboard.range24Hours') },
  { value: '7d' as const, label: t('dashboard.range7Days') },
  { value: '14d' as const, label: t('dashboard.range14Days') },
  { value: '30d' as const, label: t('dashboard.range30Days') }
])

const isTimeRangeActive = (value: TimeRangeTab): boolean => {
  return timeRange.value === value
}

const selectTimeRange = (value: TimeRangeTab) => {
  timeRange.value = value

  const now = new Date()
  if (value === '24h') {
    const start = new Date(now)
    start.setHours(start.getHours() - 24)
    startDate.value = formatLocalDate(start)
    endDate.value = formatLocalDate(now)
    loadChartData()
    return
  }

  const days = value === '7d' ? 7 : value === '14d' ? 14 : 30
  const start = new Date(now)
  start.setDate(start.getDate() - (days - 1))
  startDate.value = formatLocalDate(start)
  endDate.value = formatLocalDate(now)
  loadChartData()
}

const granularity = computed<'day' | 'hour'>(() => {
  return timeRange.value === '24h' ? 'hour' : 'day'
})

// Dark mode detection
const isDarkMode = computed(() => {
  return document.documentElement.classList.contains('dark')
})

// Chart colors
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

const shouldUseKUnitByDefault = computed(() => {
  if (!trendData.value?.length) return true
  return trendData.value.every(
    (d) =>
      toNonNegativeToken(d.input_tokens) === 0 &&
      toNonNegativeToken(d.output_tokens) === 0 &&
      toNonNegativeToken(d.cache_tokens) === 0
  )
})

// Doughnut chart options
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

// Line chart options
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
          size: 13
        }
      }
    },
    tooltip: {
      bodyFont: {
        size: 13
      },
      titleFont: {
        size: 13
      },
      callbacks: {
        label: (context: any) => {
          return `${context.dataset.label}: ${formatTokens(
            context.raw,
            shouldUseKUnitByDefault.value ? 'K' : undefined
          )}`
        },
        footer: (tooltipItems: any) => {
          const dataIndex = tooltipItems[0]?.dataIndex
          if (dataIndex !== undefined && trendData.value[dataIndex]) {
            const data = trendData.value[dataIndex]
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
          size: 12
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
          size: 12
        },
        callback: (value: string | number) =>
          formatTokens(Number(value), shouldUseKUnitByDefault.value ? 'K' : undefined)
      }
    }
  }
}))

// Model chart data
const modelChartData = computed(() => {
  if (!modelStats.value?.length) return null

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

  const backgroundColor = modelStats.value.map((_, index) => colors[index % colors.length])

  return {
    labels: modelStats.value.map((m) => m.model),
    datasets: [
      {
        data: modelStats.value.map((m) => m.total_tokens),
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
  return modelChartData.value.labels.map((label, index) => ({
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

// Trend chart data
const trendChartData = computed(() => {
  if (!trendData.value?.length) return null

  const formatTrendLabel = (value: string): string => {
    if (granularity.value !== 'hour') return value
    const parts = value.split(' ')
    if (parts.length >= 2 && parts[1]) return parts[1]
    if (value.length >= 5) return value.slice(-5)
    return value
  }

  return {
    labels: trendData.value.map((d) => formatTrendLabel(d.date)),
    datasets: [
      {
        label: 'Input',
        data: trendData.value.map((d) => toNonNegativeToken(d.input_tokens)),
        borderColor: chartColors.value.input,
        backgroundColor: `${chartColors.value.input}20`,
        fill: true,
        tension: 0.3
      },
      {
        label: 'Output',
        data: trendData.value.map((d) => toNonNegativeToken(d.output_tokens)),
        borderColor: chartColors.value.output,
        backgroundColor: `${chartColors.value.output}20`,
        fill: true,
        tension: 0.3
      },
      {
        label: 'Cache',
        data: trendData.value.map((d) => toNonNegativeToken(d.cache_tokens)),
        borderColor: chartColors.value.cache,
        backgroundColor: `${chartColors.value.cache}20`,
        fill: true,
        tension: 0.3
      }
    ]
  }
})

// Format helpers
const formatTokens = (value: number | undefined, forceUnit?: 'K'): string => {
  if (value === undefined || value === null) return forceUnit === 'K' ? '0K' : '0'
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

const formatDuration = (ms: number): string => {
  if (ms >= 1000) {
    return `${(ms / 1000).toFixed(2)}s`
  }
  return `${Math.round(ms)}ms`
}

const navigateTo = (path: string) => {
  router.push(path)
}

// Load data
const loadDashboardStats = async () => {
  loading.value = true
  try {
    await authStore.refreshUser()
    stats.value = await usageAPI.getDashboardStats()
  } catch (error) {
    console.error('Error loading dashboard stats:', error)
  } finally {
    loading.value = false
  }
}

const loadChartData = async () => {
  loadingCharts.value = true
  try {
    const params: UserTrendParams = { granularity: granularity.value }
    if (granularity.value === 'hour') {
      params.hours = 24
    } else {
      params.start_date = startDate.value
      params.end_date = endDate.value
    }

    const [trendResponse, modelResponse] = await Promise.all([
      usageAPI.getDashboardTrend(params),
      usageAPI.getDashboardModels(params)
    ])

    // Ensure we always have arrays, even if API returns null
    trendData.value = trendResponse.trend || []
    modelStats.value = modelResponse.models || []
  } catch (error) {
    console.error('Error loading chart data:', error)
  } finally {
    loadingCharts.value = false
  }
}

const loadRecentUsage = async () => {
  loadingUsage.value = true
  try {
    // Use local timezone instead of UTC
    const now = new Date()
    const endDate = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`
    const weekAgo = new Date(Date.now() - 7 * 24 * 60 * 60 * 1000)
    const startDate = `${weekAgo.getFullYear()}-${String(weekAgo.getMonth() + 1).padStart(2, '0')}-${String(weekAgo.getDate()).padStart(2, '0')}`
    const usageResponse = await usageAPI.getByDateRange(startDate, endDate)
    recentUsage.value = usageResponse.items.slice(0, 5)
  } catch (error) {
    console.error('Failed to load recent usage:', error)
  } finally {
    loadingUsage.value = false
  }
}

onMounted(async () => {
  // Load critical data first
  await loadDashboardStats()

  // Force refresh subscription status when entering dashboard (bypass cache)
  subscriptionStore.fetchActiveSubscriptions(true).catch((error) => {
    console.error('Failed to refresh subscription status:', error)
  })

  // Load chart data and recent usage in parallel (non-critical)
  Promise.all([loadChartData(), loadRecentUsage(), appStore.fetchPublicSettings()]).catch((error) => {
    console.error('Error loading secondary data:', error)
  })
})

// Watch for dark mode changes
watch(isDarkMode, () => {
  nextTick(() => {
    modelChartRef.value?.chart?.update()
    trendChartRef.value?.chart?.update()
  })
})
</script>
