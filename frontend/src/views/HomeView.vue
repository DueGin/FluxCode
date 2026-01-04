<template>
  <div class="min-h-screen bg-[#faf7f2] text-gray-900 dark:bg-dark-950 dark:text-gray-100">
    <PublicHeader :site-name="siteName" :site-logo="siteLogo" />

    <main id="top" class="pt-24">
      <!-- Hero（参考 HOPCC：居中排版 + 暖色背景） -->
      <section class="relative overflow-hidden">
        <div class="pointer-events-none absolute inset-0">
          <div class="absolute -top-56 right-[-120px] h-[520px] w-[520px] rounded-full bg-primary-500/15 blur-3xl dark:bg-primary-500/10"></div>
          <div class="absolute -bottom-56 left-[-160px] h-[520px] w-[520px] rounded-full bg-blue-500/10 blur-3xl dark:bg-blue-500/10"></div>
          <div class="absolute inset-0 opacity-[0.35] [background-image:radial-gradient(rgba(0,0,0,0.06)_1px,transparent_1px)] [background-size:24px_24px] dark:opacity-[0.08]"></div>
        </div>

        <div class="relative mx-auto max-w-4xl px-6 py-24 text-center sm:py-28">
          <h1 class="text-6xl font-extrabold tracking-tight text-gray-900 dark:text-white sm:text-7xl">
            <span class="relative inline-block">
              <!-- 标题右上渐变光晕 -->
              <span
                aria-hidden="true"
                class="pointer-events-none absolute -right-16 -top-16 h-44 w-44 rounded-full bg-gradient-to-br from-primary-400/45 via-sky-400/25 to-purple-400/45 blur-3xl opacity-90 mix-blend-multiply dark:opacity-60 dark:mix-blend-screen"
              ></span>
              <span
                aria-hidden="true"
                class="pointer-events-none absolute -right-10 -top-10 h-28 w-28 rounded-full bg-gradient-to-tr from-primary-500/20 to-blue-500/20 blur-2xl opacity-90"
              ></span>
              {{ siteName }}
            </span>
          </h1>

          <p class="mt-6 text-3xl font-semibold tracking-tight text-[#7b6857] dark:text-primary-200 sm:text-5xl">
            {{ t('home.hero.headline') }}
          </p>

          <p class="mt-6 text-base leading-relaxed text-gray-600 dark:text-dark-300 sm:text-lg">
            {{ t('home.hero.description') }}
          </p>

          <div class="mt-10 flex flex-col items-center justify-center gap-3 sm:flex-row">
            <router-link
              :to="isAuthenticated ? dashboardPath : '/login'"
              class="inline-flex items-center justify-center rounded-full bg-[#7b6857] px-8 py-3 text-sm font-semibold text-white shadow-lg shadow-black/5 transition-colors hover:bg-[#6b5a4a] dark:bg-white dark:text-gray-900 dark:hover:bg-gray-200"
            >
              {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}
              <svg class="ml-2 h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M13.5 4.5L21 12m0 0l-7.5 7.5M21 12H3" />
              </svg>
            </router-link>

            <router-link
              to="/usage"
              class="inline-flex items-center justify-center rounded-full border border-[#7b6857]/30 bg-white/40 px-8 py-3 text-sm font-semibold text-[#7b6857] shadow-sm backdrop-blur transition-colors hover:bg-white/60 dark:border-white/15 dark:bg-white/5 dark:text-white/80 dark:hover:bg-white/10"
            >
              {{ t('home.hero.secondaryCta') }}
            </router-link>
          </div>

          <div class="mt-12 flex flex-wrap items-center justify-center gap-x-8 gap-y-3 text-sm text-gray-700 dark:text-dark-200">
            <div class="flex items-center gap-2">
              <svg class="h-4 w-4 text-[#7b6857] dark:text-primary-300" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                <path stroke-linecap="round" stroke-linejoin="round" d="M3 3v18h18" />
                <path stroke-linecap="round" stroke-linejoin="round" d="M7 14l4-4 3 3 6-7" />
              </svg>
              <span class="font-medium">{{ t('home.hero.stats.performance') }}</span>
            </div>
            <div class="flex items-center gap-2">
              <svg class="h-4 w-4 text-[#7b6857] dark:text-primary-300" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                <path stroke-linecap="round" stroke-linejoin="round" d="M12 3l7.5 4.5V12c0 5.25-3.5 9.75-7.5 9.75S4.5 17.25 4.5 12V7.5L12 3z" />
                <path stroke-linecap="round" stroke-linejoin="round" d="M9.5 12.5l1.75 1.75L15.5 10" />
              </svg>
              <span class="font-medium">{{ t('home.hero.stats.control') }}</span>
            </div>
            <div class="flex items-center gap-2">
              <svg class="h-4 w-4 text-[#7b6857] dark:text-primary-300" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                <path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6l4 2" />
                <path stroke-linecap="round" stroke-linejoin="round" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              <span class="font-medium">{{ t('home.hero.stats.security') }}</span>
            </div>
          </div>
        </div>
      </section>

      <!-- Features -->
      <section id="features" class="scroll-mt-28 pb-24">
        <div class="mx-auto max-w-6xl px-6">
          <div class="mb-10 text-center">
            <h2 class="text-3xl font-semibold tracking-tight text-gray-900 dark:text-white">
              {{ t('home.sections.featuresTitle') }}
            </h2>
            <p class="mt-3 text-gray-600 dark:text-dark-300">
              {{ t('home.sections.featuresSubtitle') }}
            </p>
          </div>

          <div class="grid gap-6 md:grid-cols-3">
            <div class="group rounded-3xl border border-black/5 bg-white/70 p-6 shadow-sm backdrop-blur transition-all hover:-translate-y-0.5 hover:shadow-lg hover:shadow-black/5 dark:border-white/10 dark:bg-dark-900/40">
              <div class="mb-4 flex h-12 w-12 items-center justify-center rounded-2xl bg-blue-600 text-white shadow-lg shadow-blue-600/20">
                <svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="M5.25 14.25h13.5m-13.5 0a3 3 0 01-3-3m3 3a3 3 0 100 6h13.5a3 3 0 100-6m-16.5-3a3 3 0 013-3h13.5a3 3 0 013 3m-19.5 0a4.5 4.5 0 01.9-2.7L5.737 5.1a3.375 3.375 0 012.7-1.35h7.126c1.062 0 2.062.5 2.7 1.35l2.587 3.45a4.5 4.5 0 01.9 2.7m0 0a3 3 0 01-3 3"
                  />
                </svg>
              </div>
              <h3 class="mb-2 text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('home.features.unifiedGateway') }}
              </h3>
              <p class="text-sm leading-relaxed text-gray-600 dark:text-dark-400">
                {{ t('home.features.unifiedGatewayDesc') }}
              </p>
            </div>

            <div class="group rounded-3xl border border-black/5 bg-white/70 p-6 shadow-sm backdrop-blur transition-all hover:-translate-y-0.5 hover:shadow-lg hover:shadow-black/5 dark:border-white/10 dark:bg-dark-900/40">
              <div class="mb-4 flex h-12 w-12 items-center justify-center rounded-2xl bg-primary-600 text-white shadow-lg shadow-primary-600/20">
                <svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="M18 18.72a9.094 9.094 0 003.741-.479 3 3 0 00-4.682-2.72m.94 3.198l.001.031c0 .225-.012.447-.037.666A11.944 11.944 0 0112 21c-2.17 0-4.207-.576-5.963-1.584A6.062 6.062 0 016 18.719m12 0a5.971 5.971 0 00-.941-3.197m0 0A5.995 5.995 0 0012 12.75a5.995 5.995 0 00-5.058 2.772m0 0a3 3 0 00-4.681 2.72 8.986 8.986 0 003.74.477m.94-3.197a5.971 5.971 0 00-.94 3.197M15 6.75a3 3 0 11-6 0 3.75 3.75 0 016 0zm6 3a2.25 2.25 0 11-4.5 0 2.25 2.25 0 014.5 0zm-13.5 0a2.25 2.25 0 11-4.5 0 2.25 2.25 0 014.5 0z"
                  />
                </svg>
              </div>
              <h3 class="mb-2 text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('home.features.multiAccount') }}
              </h3>
              <p class="text-sm leading-relaxed text-gray-600 dark:text-dark-400">
                {{ t('home.features.multiAccountDesc') }}
              </p>
            </div>

            <div class="group rounded-3xl border border-black/5 bg-white/70 p-6 shadow-sm backdrop-blur transition-all hover:-translate-y-0.5 hover:shadow-lg hover:shadow-black/5 dark:border-white/10 dark:bg-dark-900/40">
              <div class="mb-4 flex h-12 w-12 items-center justify-center rounded-2xl bg-purple-600 text-white shadow-lg shadow-purple-600/20">
                <svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="M2.25 18.75a60.07 60.07 0 0115.797 2.101c.727.198 1.453-.342 1.453-1.096V18.75M3.75 4.5v.75A.75.75 0 013 6h-.75m0 0v-.375c0-.621.504-1.125 1.125-1.125H20.25M2.25 6v9m18-10.5v.75c0 .414.336.75.75.75h.75m-1.5-1.5h.375c.621 0 1.125.504 1.125 1.125v9.75c0 .621-.504 1.125-1.125 1.125h-.375m1.5-1.5H21a.75.75 0 00-.75.75v.75m0 0H3.75m0 0h-.375a1.125 1.125 0 01-1.125-1.125V15m1.5 1.5v-.75A.75.75 0 003 15h-.75M15 10.5a3 3 0 11-6 0 3 3 0 016 0z"
                  />
                </svg>
              </div>
              <h3 class="mb-2 text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('home.features.balanceQuota') }}
              </h3>
              <p class="text-sm leading-relaxed text-gray-600 dark:text-dark-400">
                {{ t('home.features.balanceQuotaDesc') }}
              </p>
            </div>
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
          <a
            :href="githubUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-sm text-gray-500 transition-colors hover:text-gray-700 dark:text-dark-400 dark:hover:text-white"
          >
            GitHub
          </a>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore, useAuthStore } from '@/stores'
import PublicHeader from '@/components/layout/PublicHeader.vue'

const { t } = useI18n()

const appStore = useAppStore()
const authStore = useAuthStore()

// Site settings
const siteName = computed(() => appStore.siteName || 'Sub2API')
const siteLogo = computed(() => appStore.siteLogo || '')

// GitHub URL
const githubUrl = 'https://github.com/Wei-Shaw/sub2api'

// Auth state
const isAuthenticated = computed(() => authStore.isAuthenticated)
const dashboardPath = computed(() => (authStore.isAdmin ? '/admin/dashboard' : '/dashboard'))

// Current year for footer
const currentYear = computed(() => new Date().getFullYear())

onMounted(() => {
  authStore.checkAuth()
  appStore.fetchPublicSettings()
})
</script>
