<script setup lang="ts">
import { rawFileUrl } from '@/api/client'
import type { FsItem } from '@/api/types'
import { formatBytes, formatModifiedAt } from '@/utils/format'

defineProps<{ item: FsItem }>()
</script>

<template>
  <section class="unsupported-viewer">
    <div class="generic-file-icon" aria-hidden="true">FILE</div>
    <p class="eyebrow">NO PREVIEW</p>
    <h2>{{ item.name }}</h2>
    <p>此文件类型暂不支持在线预览，你仍可下载原文件。</p>
    <dl class="file-facts">
      <div>
        <dt>类型</dt>
        <dd>{{ item.mime || '未知' }}</dd>
      </div>
      <div>
        <dt>大小</dt>
        <dd>{{ formatBytes(item.size) }}</dd>
      </div>
      <div>
        <dt>修改时间</dt>
        <dd>{{ formatModifiedAt(item.modifiedAt) }}</dd>
      </div>
    </dl>
    <a class="primary-button" :href="rawFileUrl(item.path, true)">下载文件</a>
  </section>
</template>
