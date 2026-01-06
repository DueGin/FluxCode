<template>
  <div class="min-h-screen bg-[#faf7f2] text-gray-900 dark:bg-dark-950 dark:text-gray-100">
    <PublicHeader :site-name="siteName" :site-logo="siteLogo" />

    <main class="pt-24">
      <section class="mx-auto max-w-6xl px-6 py-20">
        <div class="max-w-2xl">
          <h1 class="text-4xl font-semibold tracking-tight text-gray-900 dark:text-white sm:text-5xl">
            {{ t('home.sections.pricingTitle') }}
          </h1>
          <p class="mt-4 text-base leading-relaxed text-gray-600 dark:text-dark-300">
            {{ t('home.sections.pricingSubtitle') }}
          </p>
        </div>

        <div class="mt-12 grid gap-6 md:grid-cols-3">
          <div class="rounded-3xl border border-black/5 bg-white/70 p-6 shadow-sm backdrop-blur dark:border-white/10 dark:bg-dark-900/40">
            <div class="flex items-center gap-3">
              <div
                class="flex h-11 w-11 items-center justify-center rounded-2xl bg-blue-600 text-white shadow-lg shadow-blue-600/20"
              >
                <svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M3 3v18h18" />
                  <path stroke-linecap="round" stroke-linejoin="round" d="M7 14l4-4 3 3 6-7" />
                </svg>
              </div>
              <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('home.pricing.usageTitle') }}
              </h3>
            </div>
            <p class="mt-3 text-sm leading-relaxed text-gray-600 dark:text-dark-400">
              {{ t('home.pricing.usageDesc') }}
            </p>
          </div>

          <div class="rounded-3xl border border-black/5 bg-white/70 p-6 shadow-sm backdrop-blur dark:border-white/10 dark:bg-dark-900/40">
            <div class="flex items-center gap-3">
              <div
                class="flex h-11 w-11 items-center justify-center rounded-2xl bg-primary-600 text-white shadow-lg shadow-primary-600/20"
              >
                <svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="M12 3l7.5 4.5V12c0 5.25-3.5 9.75-7.5 9.75S4.5 17.25 4.5 12V7.5L12 3z"
                  />
                  <path stroke-linecap="round" stroke-linejoin="round" d="M9.5 12.5l1.75 1.75L15.5 10" />
                </svg>
              </div>
              <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('home.pricing.quotaTitle') }}
              </h3>
            </div>
            <p class="mt-3 text-sm leading-relaxed text-gray-600 dark:text-dark-400">
              {{ t('home.pricing.quotaDesc') }}
            </p>
          </div>

          <div class="rounded-3xl border border-black/5 bg-white/70 p-6 shadow-sm backdrop-blur dark:border-white/10 dark:bg-dark-900/40">
            <div class="flex items-center gap-3">
              <div
                class="flex h-11 w-11 items-center justify-center rounded-2xl bg-purple-600 text-white shadow-lg shadow-purple-600/20"
              >
                <svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="M9 5h6a2 2 0 012 2v1a5 5 0 01-10 0V7a2 2 0 012-2z"
                  />
                  <path stroke-linecap="round" stroke-linejoin="round" d="M7 9v10a2 2 0 002 2h6a2 2 0 002-2V9" />
                  <path stroke-linecap="round" stroke-linejoin="round" d="M10 13h4" />
                </svg>
              </div>
              <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('home.pricing.redeemTitle') }}
              </h3>
            </div>
            <p class="mt-3 text-sm leading-relaxed text-gray-600 dark:text-dark-400">
              {{ t('home.pricing.redeemDesc') }}
            </p>
          </div>
        </div>
      </section>
    </main>

    <footer class="border-t border-black/5 bg-[#faf7f2] px-6 py-10 dark:border-white/10 dark:bg-dark-950">
      <div class="mx-auto flex max-w-6xl flex-col items-center justify-between gap-4 text-center sm:flex-row sm:text-left">
        <p class="text-sm text-gray-500 dark:text-dark-400">
          &copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}
        </p>
        <div class="flex items-center gap-4">
          <!-- <a
            :href="githubUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-sm text-gray-500 transition-colors hover:text-gray-700 dark:text-dark-400 dark:hover:text-white"
          >
            GitHub
          </a> -->
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import PublicHeader from '@/components/layout/PublicHeader.vue'

const { t } = useI18n()

const appStore = useAppStore()

// Site settings
const siteName = computed(() => appStore.siteName || 'FluxCode')
const siteLogo = computed(() => appStore.siteLogo || '')

// GitHub URL
const githubUrl = 'https://github.com/DueGin/FluxCode'

// Current year for footer
const currentYear = computed(() => new Date().getFullYear())

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>
