<template>
  <div class="min-h-screen bg-[#faf7f2] text-gray-900 dark:bg-dark-950 dark:text-gray-100">
    <PublicHeader :site-name="siteName" :site-logo="siteLogo" />

    <main class="pt-24">
      <section class="mx-auto max-w-6xl px-6 py-20">
        <div class="max-w-2xl">
          <h1 class="text-4xl font-semibold tracking-tight text-gray-900 dark:text-white sm:text-5xl">
            {{ t('home.sections.docsTitle') }}
          </h1>
          <p class="mt-4 text-base leading-relaxed text-gray-600 dark:text-dark-300">
            {{ t('home.sections.docsSubtitle') }}
          </p>
        </div>

        <div class="mt-12">
          <div class="mb-10 flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
            <div>
              <h3 class="text-xl font-semibold tracking-tight text-gray-900 dark:text-white">
                {{ t('home.quickstart.title') }}
              </h3>
              <p class="mt-2 text-sm text-gray-600 dark:text-dark-400">
                {{ t('home.quickstart.subtitle') }}
              </p>
            </div>
            <router-link to="/home" class="btn btn-secondary btn-sm w-full justify-center sm:w-auto">
              {{ t('home.nav.features') }}
            </router-link>
          </div>

          <div class="grid gap-4 md:grid-cols-3">
            <div class="rounded-3xl border border-black/5 bg-white/70 p-6 shadow-sm backdrop-blur dark:border-white/10 dark:bg-dark-900/40">
              <div class="flex items-center gap-3">
                <span
                  class="flex h-8 w-8 items-center justify-center rounded-full bg-primary-500/10 text-sm font-semibold text-primary-700 dark:text-primary-300"
                  >1</span
                >
                <h4 class="text-sm font-semibold text-gray-900 dark:text-white">
                  {{ t('home.quickstart.steps.createKey.title') }}
                </h4>
              </div>
              <p class="mt-3 text-sm leading-relaxed text-gray-600 dark:text-dark-400">
                {{ t('home.quickstart.steps.createKey.desc') }}
              </p>
            </div>

            <div class="rounded-3xl border border-black/5 bg-white/70 p-6 shadow-sm backdrop-blur dark:border-white/10 dark:bg-dark-900/40">
              <div class="flex items-center gap-3">
                <span
                  class="flex h-8 w-8 items-center justify-center rounded-full bg-primary-500/10 text-sm font-semibold text-primary-700 dark:text-primary-300"
                  >2</span
                >
                <h4 class="text-sm font-semibold text-gray-900 dark:text-white">
                  {{ t('home.quickstart.steps.chooseApi.title') }}
                </h4>
              </div>
              <p class="mt-3 text-sm leading-relaxed text-gray-600 dark:text-dark-400">
                {{ t('home.quickstart.steps.chooseApi.desc') }}
              </p>
            </div>

            <div class="rounded-3xl border border-black/5 bg-white/70 p-6 shadow-sm backdrop-blur dark:border-white/10 dark:bg-dark-900/40">
              <div class="flex items-center gap-3">
                <span
                  class="flex h-8 w-8 items-center justify-center rounded-full bg-primary-500/10 text-sm font-semibold text-primary-700 dark:text-primary-300"
                  >3</span
                >
                <h4 class="text-sm font-semibold text-gray-900 dark:text-white">
                  {{ t('home.quickstart.steps.call.title') }}
                </h4>
              </div>
              <p class="mt-3 text-sm leading-relaxed text-gray-600 dark:text-dark-400">
                {{ t('home.quickstart.steps.call.desc') }}
              </p>
            </div>
          </div>
        </div>

        <div class="mt-14 grid gap-6 md:grid-cols-2">
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="rounded-3xl border border-black/5 bg-white/70 p-6 shadow-sm backdrop-blur transition-all hover:-translate-y-0.5 hover:shadow-lg hover:shadow-black/5 dark:border-white/10 dark:bg-dark-900/40"
          >
            <div class="flex items-center justify-between">
              <div>
                <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('home.docs') }}</h3>
                <p class="mt-2 text-sm text-gray-600 dark:text-dark-400">{{ t('home.viewDocs') }}</p>
              </div>
              <svg class="h-5 w-5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M4.5 19.5l15-15m0 0H8.25m11.25 0v11.25" />
              </svg>
            </div>
          </a>

          <a
            :href="githubUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="rounded-3xl border border-black/5 bg-white/70 p-6 shadow-sm backdrop-blur transition-all hover:-translate-y-0.5 hover:shadow-lg hover:shadow-black/5 dark:border-white/10 dark:bg-dark-900/40"
          >
            <div class="flex items-center justify-between">
              <div>
                <h3 class="text-lg font-semibold text-gray-900 dark:text-white">GitHub</h3>
                <p class="mt-2 text-sm text-gray-600 dark:text-dark-400">{{ t('home.viewOnGithub') }}</p>
              </div>
              <svg class="h-5 w-5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M4.5 19.5l15-15m0 0H8.25m11.25 0v11.25" />
              </svg>
            </div>
          </a>
        </div>
      </section>
    </main>
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
const docUrl = computed(() => appStore.docUrl || '')

// GitHub URL
const githubUrl = 'https://github.com/DueGin/FluxCode'

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>
