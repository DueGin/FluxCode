<template>
  <BaseDialog :show="show" :title="dialogTitle" width="wide" :close-on-click-outside="true" @close="handleClose">
    <div class="space-y-5">
      <div class="qq-popup-markdown" v-html="renderedHtml"></div>

      <label v-if="checkboxLabel" class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
        <input v-model="suppressChecked" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
        <span>{{ checkboxLabel }}</span>
      </label>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button
          type="button"
          class="btn btn-secondary"
          :disabled="savingPreference"
          @click="handleClose"
        >
          {{ t('common.close') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { renderMarkdownToHtml } from '@/utils/markdown'
import { useAppStore, useAuthStore } from '@/stores'
import { userAPI } from '@/api'

type PopupContext = 'public' | 'dashboard'

const PUBLIC_PATHS = new Set(['/home', '/docs', '/pricing'])
const DASHBOARD_PATHS = new Set(['/dashboard', '/keys', '/usage', '/redeem', '/profile', '/subscriptions'])

const LOCAL_KEY_PUBLIC_DISMISSED_DATE = 'qq-group-popup:public:dismissed-date'
const SESSION_KEY_PUBLIC_DISMISSED = 'qq-group-popup:public:session-dismissed'
const SESSION_KEY_DASHBOARD_DISMISSED = 'qq-group-popup:dashboard:session-dismissed'

const route = useRoute()
const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const show = ref(false)
const context = ref<PopupContext | null>(null)
const suppressChecked = ref(false)

const savingPreference = ref(false)
const dashboardPopupDisabled = ref<boolean | null>(null)
const dashboardPrefLoading = ref(false)

function getTodayLocalISODate(): string {
  const now = new Date()
  const y = now.getFullYear()
  const m = String(now.getMonth() + 1).padStart(2, '0')
  const d = String(now.getDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}

function isPublicDismissedToday(): boolean {
  return localStorage.getItem(LOCAL_KEY_PUBLIC_DISMISSED_DATE) === getTodayLocalISODate()
}

function setPublicDismissedToday(): void {
  localStorage.setItem(LOCAL_KEY_PUBLIC_DISMISSED_DATE, getTodayLocalISODate())
}

function isSessionDismissed(key: string): boolean {
  return sessionStorage.getItem(key) === '1'
}

function setSessionDismissed(key: string): void {
  sessionStorage.setItem(key, '1')
}

async function loadDashboardPreferenceIfNeeded(): Promise<void> {
  if (!authStore.isAuthenticated || authStore.isAdmin) {
    dashboardPopupDisabled.value = null
    return
  }
  if (dashboardPrefLoading.value) return
  if (dashboardPopupDisabled.value !== null) return

  dashboardPrefLoading.value = true
  try {
    const prefs = await userAPI.getUiPreferences()
    dashboardPopupDisabled.value = !!prefs.dashboard_qq_group_popup_disabled
  } catch {
    dashboardPopupDisabled.value = false
  } finally {
    dashboardPrefLoading.value = false
  }
}

const dialogTitle = computed(() => appStore.qqGroupPopupTitle || t('qqGroupPopup.title'))
const markdown = computed(() => (appStore.qqGroupPopupMarkdown || '').trim())
const renderedHtml = computed(() => renderMarkdownToHtml(markdown.value))

const checkboxLabel = computed(() => {
  if (context.value === 'public') return t('qqGroupPopup.dismissToday')
  if (context.value === 'dashboard') return t('qqGroupPopup.dismissForever')
  return ''
})

async function maybeShow(): Promise<void> {
  if (show.value) return
  if (!markdown.value) return

  const path = route.path

  if (PUBLIC_PATHS.has(path)) {
    if (isPublicDismissedToday()) return
    if (isSessionDismissed(SESSION_KEY_PUBLIC_DISMISSED)) return
    context.value = 'public'
    suppressChecked.value = false
    show.value = true
    return
  }

  if (DASHBOARD_PATHS.has(path)) {
    if (!authStore.isAuthenticated || authStore.isAdmin) return
    if (isSessionDismissed(SESSION_KEY_DASHBOARD_DISMISSED)) return
    await loadDashboardPreferenceIfNeeded()
    if (dashboardPopupDisabled.value) return
    context.value = 'dashboard'
    suppressChecked.value = false
    show.value = true
  }
}

async function handleClose(): Promise<void> {
  if (!context.value) {
    show.value = false
    return
  }

  if (context.value === 'public') {
    if (suppressChecked.value) {
      setPublicDismissedToday()
    } else {
      setSessionDismissed(SESSION_KEY_PUBLIC_DISMISSED)
    }
  }

  if (context.value === 'dashboard') {
    setSessionDismissed(SESSION_KEY_DASHBOARD_DISMISSED)
    if (suppressChecked.value && authStore.isAuthenticated && !authStore.isAdmin) {
      savingPreference.value = true
      try {
        const updated = await userAPI.updateUiPreferences({ dashboard_qq_group_popup_disabled: true })
        dashboardPopupDisabled.value = !!updated.dashboard_qq_group_popup_disabled
      } catch (error) {
        appStore.showError((error as { message?: string }).message || t('common.error'))
      } finally {
        savingPreference.value = false
      }
    }
  }

  show.value = false
  context.value = null
  suppressChecked.value = false
}

watch(
  () => authStore.user?.id,
  () => {
    dashboardPopupDisabled.value = null
  },
  { immediate: true }
)

watch(
  () => [route.path, markdown.value, authStore.isAuthenticated, authStore.isAdmin],
  () => {
    void maybeShow()
  },
  { immediate: true }
)
</script>

<style scoped>
.qq-popup-markdown :deep(p) {
  margin: 0 0 0.75rem;
}

.qq-popup-markdown :deep(p:last-child) {
  margin-bottom: 0;
}

.qq-popup-markdown :deep(ul),
.qq-popup-markdown :deep(ol) {
  margin: 0.5rem 0 0.75rem;
  padding-left: 1.25rem;
}

.qq-popup-markdown :deep(li) {
  margin: 0.25rem 0;
}

.qq-popup-markdown :deep(a) {
  color: rgb(79 70 229);
  text-decoration: underline;
}

.qq-popup-markdown :deep(code) {
  padding: 0.1rem 0.35rem;
  border-radius: 0.375rem;
  background: rgba(15, 23, 42, 0.06);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
  font-size: 0.875em;
}

.dark .qq-popup-markdown :deep(code) {
  background: rgba(148, 163, 184, 0.12);
}
</style>
