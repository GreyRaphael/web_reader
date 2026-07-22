<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { getWorkspace, setWorkspace } from '@/api/client'
import { iconSvg } from '@/utils/icons'

const props = defineProps<{ username?: string }>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'updated', path: string): void
}>()

const workspace = ref('')
const loading = ref(false)
const saving = ref(false)
const errorMsg = ref('')
const successMsg = ref('')

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

onMounted(() => {
  fetchCurrentWorkspace()
})
</script>

<template>
  <div class="modal-backdrop" @click.self="emit('close')">
    <div class="modal-dialog" role="dialog" aria-modal="true" aria-labelledby="settings-dialog-title">
      <div class="modal-header">
        <h3 id="settings-dialog-title" class="modal-title">
          <span class="emoji-icon">⚙️</span>
          设置 (Settings)
        </h3>
        <button
          class="icon-button compact close-btn"
          type="button"
          aria-label="关闭设置"
          @click="emit('close')"
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
          支持绝对路径或 <code>~/</code> 前缀（自动展开为当前用户目录，如 <code>~/workspace</code>）。若路径不存在将自动创建。
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
          :disabled="saving"
          @click="emit('close')"
        >
          取消
        </button>
        <button
          class="button primary-button"
          type="button"
          :disabled="loading || saving"
          @click="handleSave"
        >
          {{ saving ? '保存中...' : '保存更改' }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.modal-backdrop {
  position: fixed;
  inset: 0;
  z-index: 1000;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.45);
  backdrop-filter: blur(2px);
  padding: 16px;
}

.modal-dialog {
  width: 100%;
  max-width: 520px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 12px;
  box-shadow: 0 12px 32px rgba(0, 0, 0, 0.2);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  animation: modal-fade-in 180ms ease-out;
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
</style>
