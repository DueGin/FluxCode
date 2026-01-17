<template>
  <aside
    class="sidebar"
    :class="[
      sidebarCollapsed ? 'w-[72px]' : 'w-64',
      { '-translate-x-full lg:translate-x-0': !mobileOpen }
    ]"
  >
    <!-- Logo/Brand -->
    <div class="sidebar-header">
      <!-- Custom Logo or Default Logo -->
      <div class="flex h-9 w-9 items-center justify-center overflow-hidden rounded-xl shadow-glow">
        <img :src="siteLogo || '/logo.png'" alt="Logo" class="h-full w-full object-contain" />
      </div>
      <transition name="fade">
        <div v-if="!sidebarCollapsed" class="flex flex-col">
          <span class="text-lg font-bold text-gray-900 dark:text-white">
            {{ siteName }}
          </span>
          <!-- Version Badge -->
          <VersionBadge v-if="isAdmin" :version="siteVersion" />
        </div>
      </transition>
    </div>

    <!-- Navigation -->
    <nav class="sidebar-nav scrollbar-hide">
      <!-- Admin View: Admin menu first, then personal menu -->
      <template v-if="isAdmin">
        <!-- Admin Section -->
        <div class="sidebar-section">
          <router-link
            v-for="item in adminNavItems"
            :key="item.path"
            :to="item.path"
            class="sidebar-link mb-1"
            :class="{ 'sidebar-link-active': isActive(item.path) }"
            :title="sidebarCollapsed ? item.label : undefined"
            :id="
              item.path === '/admin/accounts'
                ? 'sidebar-channel-manage'
                : item.path === '/admin/groups'
                  ? 'sidebar-group-manage'
                  : item.path === '/admin/redeem'
                    ? 'sidebar-wallet'
                    : undefined
            "
            @click="handleMenuItemClick(item.path)"
          >
            <component :is="item.icon" class="h-5 w-5 flex-shrink-0" />
            <transition name="fade">
              <span v-if="!sidebarCollapsed">{{ item.label }}</span>
            </transition>
          </router-link>
        </div>

        <!-- Personal Section for Admin (hidden in simple mode) -->
        <div v-if="!authStore.isSimpleMode" class="sidebar-section">
          <div v-if="!sidebarCollapsed" class="sidebar-section-title">
            {{ t('nav.myAccount') }}
          </div>
          <div v-else class="mx-3 my-3 h-px bg-gray-200 dark:bg-dark-700"></div>

          <router-link
            v-for="item in personalNavItems"
            :key="item.path"
            :to="item.path"
            class="sidebar-link mb-1"
            :class="{ 'sidebar-link-active': isActive(item.path) }"
            :title="sidebarCollapsed ? item.label : undefined"
            :data-tour="item.path === '/keys' ? 'sidebar-my-keys' : undefined"
            @click="handleMenuItemClick(item.path)"
          >
            <component :is="item.icon" class="h-5 w-5 flex-shrink-0" />
            <transition name="fade">
              <span v-if="!sidebarCollapsed">{{ item.label }}</span>
            </transition>
          </router-link>
        </div>
      </template>

      <!-- Regular User View -->
      <template v-else>
        <div class="sidebar-section">
          <router-link
            v-for="item in userNavItems"
            :key="item.path"
            :to="item.path"
            class="sidebar-link mb-1"
            :class="{ 'sidebar-link-active': isActive(item.path) }"
            :title="sidebarCollapsed ? item.label : undefined"
            :data-tour="item.path === '/keys' ? 'sidebar-my-keys' : undefined"
            @click="handleMenuItemClick(item.path)"
          >
            <component :is="item.icon" class="h-5 w-5 flex-shrink-0" />
            <transition name="fade">
              <span v-if="!sidebarCollapsed">{{ item.label }}</span>
            </transition>
          </router-link>
        </div>
      </template>
    </nav>

    <!-- Bottom Section -->
    <div class="mt-auto border-t border-gray-100 p-3 dark:border-dark-800">
      <!-- Theme Toggle -->
      <button
        @click="toggleTheme"
        class="sidebar-link mb-2 w-full"
        :title="sidebarCollapsed ? (isDark ? t('nav.lightMode') : t('nav.darkMode')) : undefined"
      >
        <SunIcon v-if="isDark" class="h-5 w-5 flex-shrink-0 text-amber-500" />
        <MoonIcon v-else class="h-5 w-5 flex-shrink-0" />
        <transition name="fade">
          <span v-if="!sidebarCollapsed">{{
            isDark ? t('nav.lightMode') : t('nav.darkMode')
          }}</span>
        </transition>
      </button>

      <!-- Collapse Button -->
      <button
        @click="toggleSidebar"
        class="sidebar-link w-full"
        :title="sidebarCollapsed ? t('nav.expand') : t('nav.collapse')"
      >
        <ChevronDoubleLeftIcon v-if="!sidebarCollapsed" class="h-5 w-5 flex-shrink-0" />
        <ChevronDoubleRightIcon v-else class="h-5 w-5 flex-shrink-0" />
        <transition name="fade">
          <span v-if="!sidebarCollapsed">{{ t('nav.collapse') }}</span>
        </transition>
      </button>
    </div>
  </aside>

  <!-- Mobile Overlay -->
  <transition name="fade">
    <div
      v-if="mobileOpen"
      class="fixed inset-0 z-30 bg-black/50 lg:hidden"
      @click="closeMobile"
    ></div>
  </transition>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore, useAuthStore, useOnboardingStore } from '@/stores'
import VersionBadge from '@/components/common/VersionBadge.vue'
import {
  Squares2X2Icon as DashboardIcon,
  KeyIcon,
  ChartBarIcon as ChartIcon,
  GiftIcon,
  UserIcon,
  UsersIcon,
  FolderIcon,
  CreditCardIcon,
  CurrencyYenIcon,
  GlobeAltIcon as GlobeIcon,
  ServerIcon,
  TicketIcon,
  Cog6ToothIcon as CogIcon,
  SunIcon,
  MoonIcon,
  ChevronDoubleLeftIcon,
  ChevronDoubleRightIcon
} from '@heroicons/vue/24/outline'

const { t } = useI18n()

const route = useRoute()
const appStore = useAppStore()
const authStore = useAuthStore()
const onboardingStore = useOnboardingStore()

const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
const mobileOpen = computed(() => appStore.mobileOpen)
const isAdmin = computed(() => authStore.isAdmin)
const isDark = ref(document.documentElement.classList.contains('dark'))

// Site settings from appStore (cached, no flicker)
const siteName = computed(() => appStore.siteName)
const siteLogo = computed(() => appStore.siteLogo)
const siteVersion = computed(() => appStore.siteVersion)

// User navigation items (for regular users)
const userNavItems = computed(() => {
  const items = [
    { path: '/dashboard', label: t('nav.dashboard'), icon: DashboardIcon },
    { path: '/keys', label: t('nav.apiKeys'), icon: KeyIcon },
    { path: '/usage', label: t('nav.usage'), icon: ChartIcon, hideInSimpleMode: true },
    { path: '/subscriptions', label: t('nav.mySubscriptions'), icon: CreditCardIcon, hideInSimpleMode: true },
    { path: '/redeem', label: t('nav.redeem'), icon: GiftIcon, hideInSimpleMode: true },
    { path: '/profile', label: t('nav.profile'), icon: UserIcon }
  ]
  return authStore.isSimpleMode ? items.filter(item => !item.hideInSimpleMode) : items
})

// Personal navigation items (for admin's "My Account" section, without Dashboard)
const personalNavItems = computed(() => {
  const items = [
    { path: '/keys', label: t('nav.apiKeys'), icon: KeyIcon },
    { path: '/usage', label: t('nav.usage'), icon: ChartIcon, hideInSimpleMode: true },
    { path: '/subscriptions', label: t('nav.mySubscriptions'), icon: CreditCardIcon, hideInSimpleMode: true },
    { path: '/redeem', label: t('nav.redeem'), icon: GiftIcon, hideInSimpleMode: true },
    { path: '/profile', label: t('nav.profile'), icon: UserIcon }
  ]
  return authStore.isSimpleMode ? items.filter(item => !item.hideInSimpleMode) : items
})

// Admin navigation items
const adminNavItems = computed(() => {
  const baseItems = [
    { path: '/admin/dashboard', label: t('nav.dashboard'), icon: DashboardIcon },
    { path: '/admin/users', label: t('nav.users'), icon: UsersIcon, hideInSimpleMode: true },
    { path: '/admin/groups', label: t('nav.groups'), icon: FolderIcon, hideInSimpleMode: true },
    {
      path: '/admin/pricing-plans',
      label: t('nav.pricingPlans'),
      icon: CurrencyYenIcon,
      hideInSimpleMode: true
    },
    { path: '/admin/subscriptions', label: t('nav.subscriptions'), icon: CreditCardIcon, hideInSimpleMode: true },
    { path: '/admin/accounts', label: t('nav.accounts'), icon: GlobeIcon },
    { path: '/admin/proxies', label: t('nav.proxies'), icon: ServerIcon },
    { path: '/admin/redeem', label: t('nav.redeemCodes'), icon: TicketIcon, hideInSimpleMode: true },
    { path: '/admin/usage', label: t('nav.usage'), icon: ChartIcon },
  ]

  // 简单模式下，在系统设置前插入 API密钥
  if (authStore.isSimpleMode) {
    const filtered = baseItems.filter(item => !item.hideInSimpleMode)
    filtered.push({ path: '/keys', label: t('nav.apiKeys'), icon: KeyIcon })
    filtered.push({ path: '/admin/settings', label: t('nav.settings'), icon: CogIcon })
    return filtered
  }

  baseItems.push({ path: '/admin/settings', label: t('nav.settings'), icon: CogIcon })
  return baseItems
})

function toggleSidebar() {
  appStore.toggleSidebar()
}

function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

function closeMobile() {
  appStore.setMobileOpen(false)
}

function handleMenuItemClick(itemPath: string) {
  if (mobileOpen.value) {
    setTimeout(() => {
      appStore.setMobileOpen(false)
    }, 150)
  }

  // Map paths to tour selectors
  const pathToSelector: Record<string, string> = {
    '/admin/groups': '#sidebar-group-manage',
    '/admin/accounts': '#sidebar-channel-manage',
    '/keys': '[data-tour="sidebar-my-keys"]'
  }

  const selector = pathToSelector[itemPath]
  if (selector && onboardingStore.isCurrentStep(selector)) {
    onboardingStore.nextStep(500)
  }
}

function isActive(path: string): boolean {
  return route.path === path || route.path.startsWith(path + '/')
}

// Initialize theme
const savedTheme = localStorage.getItem('theme')
if (
  savedTheme === 'dark' ||
  (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
) {
  isDark.value = true
  document.documentElement.classList.add('dark')
}
</script>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
