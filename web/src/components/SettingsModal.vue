<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import {
  getSettings,
  getWorkspace,
  setWorkspace,
  setSettings,
  checkUpdate,
  applyUpdate,
  checkHealth,
} from '@/api/client'
import type { UpdateCheckResponse } from '@/api/types'
import { iconSvg } from '@/utils/icons'
import PathBrowser from './PathBrowser.vue'

const props = defineProps<{ username?: string }>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'updated', path: string): void
  (e: 'settingsChanged', enableTerminal: boolean): void
}>()

const workspace = ref('')
const loading = ref(false)
const saving = ref(false)
const errorMsg = ref('')
const successMsg = ref('')
const dialogEl = ref<HTMLDialogElement | null>(null)
let previouslyFocused: HTMLElement | null = null

const enableTerminal = ref(true)
const terminalSaving = ref(false)

const showBrowser = ref(false)

// Version & Update state
const updateInfo = ref<UpdateCheckResponse | null>(null)
const checkingUpdate = ref(false)
const updating = ref(false)
const updateError = ref('')
const updateSuccess = ref('')
const restarting = ref(false)
const showReleaseNotes = ref(false)
const showConfirmModal = ref(false)
let healthPollTimer: ReturnType<typeof setInterval> | null = null

function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`
}

function formatDate(isoStr: string): string {
  if (!isoStr) return ''
  try {
    const d = new Date(isoStr)
    return d.toLocaleDateString(undefined, { year: 'numeric', month: '2-digit', day: '2-digit' })
  } catch {
    return isoStr
  }
}

async function handleCheckUpdate() {
  checkingUpdate.value = true
  updateError.value = ''
  updateSuccess.value = ''
  showConfirmModal.value = false
  try {
    const res = await checkUpdate()
    updateInfo.value = res
  } catch (err) {
    updateError.value = err instanceof Error ? err.message : '检查更新失败'
  } finally {
    checkingUpdate.value = false
  }
}

function confirmApplyUpdate() {
  showConfirmModal.value = true
}

async function handleApplyUpdate() {
  showConfirmModal.value = false
  updating.value = true
  updateError.value = ''
  updateSuccess.value = ''
  try {
    const res = await applyUpdate()
    updateSuccess.value = res.message || '更新成功，服务正在重启...'
    startRestartHealthPolling()
  } catch (err) {
    updateError.value = err instanceof Error ? err.message : '自动更新失败'
    updating.value = false
  }
}

function startRestartHealthPolling() {
  restarting.value = true
  let attempts = 0
  const maxAttempts = 30

  setTimeout(() => {
    healthPollTimer = setInterval(async () => {
      attempts++
      const ok = await checkHealth()
      if (ok) {
        if (healthPollTimer) {
          clearInterval(healthPollTimer)
          healthPollTimer = null
        }
        updateSuccess.value = '新版本服务已就绪，正在刷新页面...'
        setTimeout(() => {
          if (typeof window !== 'undefined') {
            window.location.reload()
          }
        }, 600)
      } else if (attempts >= maxAttempts) {
        if (healthPollTimer) {
          clearInterval(healthPollTimer)
          healthPollTimer = null
        }
        restarting.value = false
        updating.value = false
        updateError.value = '服务重启探测超时，请检查终端日志或手动刷新页面。'
      }
    }, 1500)
  }, 2000)
}

function handleBrowserSelect(path: string) {
  workspace.value = path
  showBrowser.value = false
}

async function handleTerminalToggle() {
  terminalSaving.value = true
  errorMsg.value = ''
  try {
    const res = await setSettings(enableTerminal.value)
    enableTerminal.value = res.enableTerminal
    emit('settingsChanged', res.enableTerminal)
  } catch (err) {
    errorMsg.value = err instanceof Error ? err.message : '保存终端设置失败'
    enableTerminal.value = !enableTerminal.value
  } finally {
    terminalSaving.value = false
  }
}

function toggleTerminal() {
  enableTerminal.value = !enableTerminal.value
  handleTerminalToggle()
}

async function fetchSettings() {
  try {
    const res = await getSettings()
    enableTerminal.value = res.enableTerminal
  } catch {
    // default on if fetch fails
  }
}

async function fetchCurrentWorkspace() {
  loading.value = true
  errorMsg.value = ''
  try {
    const res = await getWorkspace()
    workspace.value = res.workspace
  } catch (err) {
    errorMsg.value = err instanceof Error ? err.message : '获取工作区配置失败'
  } finally {
    loading.value = false
  }
}

function resetDefault() {
  workspace.value = '~/workspace'
}

async function handleSave() {
  if (!workspace.value.trim()) {
    errorMsg.value = '工作区路径不能为空'
    return
  }
  saving.value = true
  errorMsg.value = ''
  successMsg.value = ''
  try {
    const res = await setWorkspace(workspace.value.trim())
    workspace.value = res.workspace
    successMsg.value = '工作区路径修改成功！'
    emit('updated', res.workspace)
    setTimeout(() => {
      emit('close')
    }, 600)
  } catch (err) {
    errorMsg.value = err instanceof Error ? err.message : '设置工作区失败'
  } finally {
    saving.value = false
  }
}

function closeDialog(): void {
  if (updating.value || restarting.value) {
    return
  }
  emit('close')
}

onMounted(async () => {
  previouslyFocused = document.activeElement instanceof HTMLElement ? document.activeElement : null
  await fetchCurrentWorkspace()
  await fetchSettings()
  await nextTick()
  const dialog = dialogEl.value
  if (dialog && !dialog.open) {
    dialog.showModal()
  }
  const focusable = dialog?.querySelector<HTMLElement>(
    'input:not([disabled]), button:not([disabled])',
  )
  focusable?.focus()
})

onBeforeUnmount(() => {
  if (healthPollTimer) {
    clearInterval(healthPollTimer)
    healthPollTimer = null
  }
  previouslyFocused?.focus()
})
</script>

<template>
  <dialog
    ref="dialogEl"
    class="modal-dialog"
    aria-labelledby="settings-dialog-title"
    @click.self="closeDialog"
    @cancel.prevent="closeDialog"
  >
    <div class="modal-inner">
      <div class="modal-header">
        <h3 id="settings-dialog-title" class="modal-title">
          <span class="emoji-icon">⚙️</span>
          设置 (Settings)
        </h3>
        <button
          class="icon-button compact close-btn"
          type="button"
          aria-label="关闭设置"
          @click="closeDialog"
        >
          ×
        </button>
      </div>

      <div class="modal-body">
        <div class="profile-card">
          <div class="avatar-circle" v-html="iconSvg('user', 18)"></div>
          <div class="profile-details">
            <span class="profile-title">当前账号 (User Profile)</span>
            <span class="profile-username">{{ props.username || 'admin' }}</span>
          </div>
        </div>

        <label for="workspace-path-input" class="form-label">
          服务器工作区绝对路径 (Server Workspace Path)
        </label>
        <p class="form-help">
          支持绝对路径或 <code>~/</code> 前缀（自动展开为当前用户目录，如
          <code>~/workspace</code>）。若路径不存在将自动创建。
        </p>

        <div class="input-group">
          <input
            id="workspace-path-input"
            v-model="workspace"
            type="text"
            class="form-input"
            placeholder="例如: ~/workspace 或 /home/username/data"
            :disabled="loading || saving"
            @keydown.enter="handleSave"
          />
          <button
            class="button secondary-button reset-btn"
            type="button"
            title="恢复默认路径 ~/workspace"
            :disabled="loading || saving"
            @click="resetDefault"
          >
            默认 ~/workspace
          </button>
          <button
            class="button secondary-button"
            type="button"
            :disabled="loading || saving"
            @click="showBrowser = !showBrowser"
          >
            {{ showBrowser ? '隐藏浏览' : '浏览…' }}
          </button>
        </div>

        <PathBrowser
          v-if="showBrowser"
          :initial-path="workspace || '~'"
          @select="handleBrowserSelect"
          @cancel="showBrowser = false"
        />

        <div class="settings-toggle-row">
          <div>
            <label class="form-label">浏览器终端 (Terminal)</label>
            <p class="form-help">开启后工具栏显示终端按钮，可在工作区路径下运行 shell。</p>
          </div>
          <button
            class="toggle-switch"
            type="button"
            role="switch"
            :aria-checked="enableTerminal"
            :disabled="terminalSaving || updating || restarting"
            :class="{ on: enableTerminal }"
            @click="toggleTerminal"
          >
            <span class="toggle-knob"></span>
          </button>
        </div>

        <div class="settings-divider"></div>

        <div class="version-update-section">
          <div class="section-header-row">
            <div>
              <label class="form-label">版本与更新 (Version & Update)</label>
              <p class="form-help">检查 GitHub 最新发布版本并支持一键更新。</p>
            </div>
            <div class="version-badge-group">
              <span class="version-badge current">
                {{ updateInfo?.currentVersion || 'v1.8.0' }}
              </span>
              <button
                class="button secondary-button compact-btn check-update-btn"
                type="button"
                :disabled="checkingUpdate || updating || restarting"
                @click="handleCheckUpdate"
              >
                <span v-if="checkingUpdate" class="spinner-sm"></span>
                {{ checkingUpdate ? '检查中…' : '检查更新' }}
              </button>
            </div>
          </div>

          <!-- 检查结果卡片 -->
          <div
            v-if="updateInfo"
            class="update-card"
            :class="{ 'has-update': updateInfo.hasUpdate }"
          >
            <div v-if="!updateInfo.hasUpdate" class="update-status-row latest">
              <span class="status-icon">✓</span>
              <span>当前已是最新版本 ({{ updateInfo.currentVersion }})</span>
            </div>

            <div v-else class="update-details">
              <div class="update-title-row">
                <div class="update-title">
                  <span class="new-tag">发现新版本</span>
                  <strong class="latest-version-name">{{ updateInfo.latestVersion }}</strong>
                </div>
                <span v-if="updateInfo.publishedAt" class="update-date">
                  发布于 {{ formatDate(updateInfo.publishedAt) }}
                </span>
              </div>

              <div v-if="updateInfo.assetSize" class="update-meta">
                安装包大小：{{ formatBytes(updateInfo.assetSize) }}
              </div>

              <!-- Release Notes 展开/折叠 -->
              <div v-if="updateInfo.releaseNotes" class="release-notes-box">
                <button
                  type="button"
                  class="notes-toggle-btn"
                  @click="showReleaseNotes = !showReleaseNotes"
                >
                  {{ showReleaseNotes ? '收起更新日志 ▲' : '查看更新日志 ▼' }}
                </button>
                <pre v-if="showReleaseNotes" class="release-notes-content">{{
                  updateInfo.releaseNotes
                }}</pre>
              </div>

              <!-- 不支持自动更新提示 -->
              <div v-if="!updateInfo.canAutoUpdate" class="auto-update-warning">
                <span class="warn-icon">⚠️</span>
                <span>{{ updateInfo.reason || '当前环境不支持自动更新' }}</span>
                <a
                  v-if="updateInfo.releaseUrl"
                  :href="updateInfo.releaseUrl"
                  target="_blank"
                  rel="noreferrer"
                  class="github-link"
                >
                  前往 GitHub 手动下载 ↗
                </a>
              </div>

              <!-- 二次确认栏 -->
              <div
                v-if="showConfirmModal && updateInfo.canAutoUpdate"
                class="confirm-update-banner"
              >
                <span>确认下载并自动替换当前运行版本？服务将在更新完成后自动重启。</span>
                <div class="confirm-btn-group">
                  <button
                    class="button secondary-button compact-btn"
                    type="button"
                    @click="showConfirmModal = false"
                  >
                    取消
                  </button>
                  <button
                    class="button primary-button compact-btn"
                    type="button"
                    @click="handleApplyUpdate"
                  >
                    确认更新
                  </button>
                </div>
              </div>

              <!-- 支持自动更新操作区 -->
              <div v-if="!showConfirmModal && updateInfo.canAutoUpdate" class="update-actions">
                <button
                  class="button primary-button apply-update-btn"
                  type="button"
                  :disabled="updating || restarting"
                  @click="confirmApplyUpdate"
                >
                  <span v-if="updating" class="spinner-sm"></span>
                  {{ updating ? '正在下载更新…' : '立即更新并重启' }}
                </button>
                <a
                  v-if="updateInfo.releaseUrl"
                  :href="updateInfo.releaseUrl"
                  target="_blank"
                  rel="noreferrer"
                  class="button secondary-button"
                >
                  发布页面 ↗
                </a>
              </div>
            </div>
          </div>

          <!-- 正在重启提示 -->
          <div v-if="restarting" class="restart-progress-card">
            <div class="spinner-sm"></div>
            <div>
              <strong>服务正在重启，请稍候…</strong>
              <p class="restart-desc">新服务启动就绪后页面将自动刷新恢复。</p>
            </div>
          </div>

          <!-- 更新错误与成功提示 -->
          <div v-if="updateError" class="alert-box error" role="alert">
            {{ updateError }}
          </div>
          <div v-if="updateSuccess" class="alert-box success" role="status">
            {{ updateSuccess }}
          </div>
        </div>

        <div v-if="errorMsg" class="alert-box error" role="alert">
          {{ errorMsg }}
        </div>
        <div v-if="successMsg" class="alert-box success" role="status">
          {{ successMsg }}
        </div>
      </div>

      <div class="modal-footer">
        <button
          class="button secondary-button"
          type="button"
          :disabled="saving || updating || restarting"
          @click="closeDialog"
        >
          取消
        </button>
        <button
          class="button primary-button"
          type="button"
          :disabled="loading || saving || updating || restarting"
          @click="handleSave"
        >
          {{ saving ? '保存中...' : '保存更改' }}
        </button>
      </div>
    </div>
  </dialog>
</template>

<style scoped>
.modal-dialog {
  width: 100%;
  max-width: 520px;
  max-height: calc(100vh - 32px);
  padding: 0;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--surface);
  box-shadow: 0 12px 32px rgba(0, 0, 0, 0.2);
  overflow: hidden;
  animation: modal-fade-in 180ms ease-out;
}

.modal-dialog::backdrop {
  background: rgba(0, 0, 0, 0.45);
  backdrop-filter: blur(2px);
}

.modal-inner {
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

@keyframes modal-fade-in {
  from {
    opacity: 0;
    transform: scale(0.96);
  }
  to {
    opacity: 1;
    transform: scale(1);
  }
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border);
}

.modal-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--foreground);
}

.close-btn {
  font-size: 20px;
  line-height: 1;
  color: var(--muted);
}

.modal-body {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.profile-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  background: var(--surface-muted, rgba(0, 0, 0, 0.03));
  border: 1px solid var(--border);
  border-radius: 8px;
  margin-bottom: 4px;
}

.avatar-circle {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border-radius: 50%;
  background: var(--surface);
  border: 1px solid var(--border);
  color: var(--primary, #3b82f6);
}

.profile-details {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.profile-title {
  font-size: 11px;
  color: var(--muted);
  font-weight: 500;
}

.profile-username {
  font-size: 14px;
  font-weight: 600;
  color: var(--foreground);
}

.form-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--foreground);
}

.form-help {
  margin: 0;
  font-size: 12px;
  color: var(--muted);
  line-height: 1.5;
}

.form-help code {
  background: var(--surface-muted, rgba(0, 0, 0, 0.05));
  padding: 2px 5px;
  border-radius: 4px;
  font-family: var(--font-mono, monospace);
}

.input-group {
  display: flex;
  gap: 8px;
}

.form-input {
  flex: 1;
  padding: 8px 12px;
  font-size: 13px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--surface);
  color: var(--foreground);
  outline: none;
  transition: border-color 150ms ease;
}

.form-input:focus {
  border-color: var(--primary, #3b82f6);
}

.reset-btn {
  white-space: nowrap;
}

.alert-box {
  padding: 8px 12px;
  border-radius: 6px;
  font-size: 13px;
  line-height: 1.4;
}

.settings-toggle-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 0;
  border-top: 1px solid var(--border);
}

.toggle-switch {
  position: relative;
  flex-shrink: 0;
  width: 44px;
  height: 24px;
  border: none;
  border-radius: 12px;
  background: var(--border-strong);
  cursor: pointer;
  transition: background-color 150ms ease;
}

.toggle-switch.on {
  background: var(--accent);
}

.toggle-knob {
  position: absolute;
  top: 2px;
  left: 2px;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: #fff;
  transition: transform 150ms ease;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
}

.toggle-switch.on .toggle-knob {
  transform: translateX(20px);
}

.toggle-switch:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.alert-box.error {
  background: #fef2f2;
  color: #dc2626;
  border: 1px solid #fecaca;
}

.alert-box.success {
  background: #f0fdf4;
  color: #16a34a;
  border: 1px solid #bbf7d0;
}

.modal-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  padding: 14px 20px;
  border-top: 1px solid var(--border);
  background: var(--surface-muted, rgba(0, 0, 0, 0.02));
}

.button {
  padding: 7px 14px;
  font-size: 13px;
  font-weight: 500;
  border-radius: 6px;
  cursor: pointer;
  border: 1px solid transparent;
  transition: all 150ms ease;
}

.primary-button {
  background: var(--primary, #2563eb);
  color: #ffffff;
}

.primary-button:hover:not(:disabled) {
  opacity: 0.9;
}

.secondary-button {
  background: var(--surface);
  border-color: var(--border);
  color: var(--foreground);
}

.secondary-button:hover:not(:disabled) {
  background: var(--surface-hover, rgba(0, 0, 0, 0.04));
}

.settings-divider {
  border-top: 1px solid var(--border);
  margin: 6px 0;
}

.version-update-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.section-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.version-badge-group {
  display: flex;
  align-items: center;
  gap: 8px;
}

.version-badge {
  font-size: 12px;
  font-weight: 600;
  padding: 3px 8px;
  border-radius: 6px;
  font-family: var(--font-mono, monospace);
  background: var(--surface-muted, rgba(0, 0, 0, 0.04));
  border: 1px solid var(--border);
  color: var(--foreground);
}

.compact-btn {
  padding: 4px 10px;
  font-size: 12px;
}

.check-update-btn {
  display: flex;
  align-items: center;
  gap: 6px;
}

.update-card {
  padding: 12px 14px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--surface-muted, rgba(0, 0, 0, 0.02));
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.update-card.has-update {
  border-color: var(--primary, #3b82f6);
  background: rgba(59, 130, 246, 0.04);
}

.update-status-row.latest {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #16a34a;
  font-size: 13px;
  font-weight: 500;
}

.status-icon {
  font-weight: bold;
}

.update-details {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.update-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.update-title {
  display: flex;
  align-items: center;
  gap: 8px;
}

.new-tag {
  font-size: 11px;
  font-weight: 700;
  background: var(--primary, #2563eb);
  color: #ffffff;
  padding: 2px 6px;
  border-radius: 4px;
}

.latest-version-name {
  font-size: 14px;
  color: var(--foreground);
}

.update-date {
  font-size: 12px;
  color: var(--muted);
}

.update-meta {
  font-size: 12px;
  color: var(--muted);
}

.release-notes-box {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.notes-toggle-btn {
  align-self: flex-start;
  background: none;
  border: none;
  font-size: 12px;
  color: var(--primary, #2563eb);
  cursor: pointer;
  padding: 0;
  text-decoration: underline;
}

.release-notes-content {
  max-height: 140px;
  overflow-y: auto;
  padding: 8px 10px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 6px;
  font-size: 12px;
  line-height: 1.45;
  white-space: pre-wrap;
  word-break: break-word;
  font-family: inherit;
  margin: 0;
  color: var(--foreground);
}

.auto-update-warning {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  background: #fffbeb;
  border: 1px solid #fef3c7;
  border-radius: 6px;
  color: #b45309;
  font-size: 12px;
}

.github-link {
  color: var(--primary, #2563eb);
  text-decoration: underline;
  margin-left: auto;
  font-weight: 500;
}

.confirm-update-banner {
  padding: 10px 12px;
  background: #fefce8;
  border: 1px solid #fef08a;
  border-radius: 6px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-size: 12px;
  color: #854d0e;
}

.confirm-btn-group {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.update-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 4px;
}

.apply-update-btn {
  display: flex;
  align-items: center;
  gap: 6px;
}

.restart-progress-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 14px;
  border-radius: 6px;
  background: rgba(59, 130, 246, 0.08);
  border: 1px solid rgba(59, 130, 246, 0.25);
  color: var(--foreground);
  font-size: 13px;
}

.restart-desc {
  margin: 2px 0 0;
  font-size: 12px;
  color: var(--muted);
}

.spinner-sm {
  width: 14px;
  height: 14px;
  border: 2px solid currentColor;
  border-top-color: transparent;
  border-radius: 50%;
  animation: spin 700ms linear infinite;
  display: inline-block;
  vertical-align: middle;
}

@keyframes spin {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}
</style>
