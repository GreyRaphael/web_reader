<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { AUTH_EXPIRED_EVENT, getSession } from '@/api/client'
import type { SessionResponse } from '@/api/types'
import ErrorBoundary from '@/components/ErrorBoundary.vue'
import LoginView from '@/views/LoginView.vue'
import ReaderView from '@/views/ReaderView.vue'
import { useTheme } from '@/composables/useTheme'

useTheme()

const checkingSession = ref(true)
const session = ref<SessionResponse>({ authenticated: false })
const startupError = ref('')

function handleAuthenticated(value: SessionResponse): void {
  session.value = value
  startupError.value = ''
}

function handleSignedOut(): void {
  session.value = { authenticated: false }
}

function handleExpired(): void {
  if (session.value.authenticated) {
    startupError.value = '会话已过期，请重新登录'
    handleSignedOut()
  }
}

onMounted(async () => {
  window.addEventListener(AUTH_EXPIRED_EVENT, handleExpired)
  try {
    session.value = await getSession()
  } catch (error) {
    startupError.value = error instanceof Error ? error.message : '无法连接服务器'
  } finally {
    checkingSession.value = false
  }
})

onBeforeUnmount(() => {
  window.removeEventListener(AUTH_EXPIRED_EVENT, handleExpired)
})
</script>

<template>
  <div v-if="checkingSession" class="app-loading" role="status" aria-live="polite">
    <div class="brand-mark">W</div>
    <span class="loading-ring" aria-hidden="true"></span>
    <p>正在连接工作区…</p>
  </div>
  <ErrorBoundary v-else :reset-key="session.username">
    <ReaderView
      v-if="session.authenticated"
      :username="session.username || '用户'"
      @signed-out="handleSignedOut"
    />
    <LoginView v-else :initial-error="startupError" @authenticated="handleAuthenticated" />
  </ErrorBoundary>
</template>
