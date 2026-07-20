<script setup lang="ts">
import { ref } from 'vue'
import { login } from '@/api/client'
import type { SessionResponse } from '@/api/types'
import ThemeControl from '@/components/ThemeControl.vue'

const props = defineProps<{ initialError?: string }>()
const emit = defineEmits<{ authenticated: [session: SessionResponse] }>()

const username = ref('admin')
const password = ref('')
const errorMessage = ref(props.initialError ?? '')
const submitting = ref(false)

async function submit(): Promise<void> {
  if (!username.value.trim() || !password.value) {
    errorMessage.value = '请输入用户名和密码'
    return
  }
  submitting.value = true
  errorMessage.value = ''
  try {
    const session = await login(username.value.trim(), password.value)
    if (!session.authenticated) throw new Error('登录未建立有效会话')
    password.value = ''
    emit('authenticated', session)
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '登录失败，请稍后重试'
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <main class="login-view">
    <div class="login-theme"><ThemeControl /></div>
    <section class="login-card" aria-labelledby="login-title">
      <div class="brand-mark" aria-hidden="true">W</div>
      <p class="eyebrow">PRIVATE WORKSPACE</p>
      <h1 id="login-title">Web Reader</h1>
      <p class="login-intro">登录后浏览和阅读工作区文件。</p>

      <form class="login-form" @submit.prevent="submit">
        <label for="username">用户名</label>
        <input
          id="username"
          v-model="username"
          name="username"
          type="text"
          autocomplete="username"
          autocapitalize="none"
          spellcheck="false"
          :disabled="submitting"
          autofocus
        />

        <label for="password">密码</label>
        <input
          id="password"
          v-model="password"
          name="password"
          type="password"
          autocomplete="current-password"
          :disabled="submitting"
        />

        <p v-if="errorMessage" class="form-error" role="alert">{{ errorMessage }}</p>
        <button class="primary-button login-button" type="submit" :disabled="submitting">
          <span v-if="submitting" class="button-spinner" aria-hidden="true"></span>
          {{ submitting ? '正在登录…' : '登录' }}
        </button>
      </form>
      <p class="login-footnote">只读访问 · 会话由服务器安全管理</p>
    </section>
  </main>
</template>
