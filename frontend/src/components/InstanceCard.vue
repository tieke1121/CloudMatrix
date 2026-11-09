<template>
  <el-card shadow="hover" class="instance-card" :class="{ 'is-selected': selected }">
    <div class="card-top">
      <div class="header-info">
        <el-checkbox :model-value="selected" :disabled="disabled" @update:model-value="onSelectionChange" />
        <span class="instance-name">{{ instance.name }}</span>
      </div>
      <el-tag :type="statusTagType" size="small" effect="dark">
        {{ instance.observed_state }}
      </el-tag>
    </div>

    <div class="card-body">
      <div class="prop-item">
        <span class="label">Serial:</span>
        <span class="val">{{ instance.adb_serial || '未绑定' }}</span>
      </div>
      <div class="prop-item">
        <span class="label">厂商ID:</span>
        <span class="val">{{ instance.provider_instance_id }}</span>
      </div>
    </div>

    <div class="card-footer">
      <el-button 
        v-if="instance.observed_state === 'stopped' || instance.observed_state === 'discovered'" 
        type="success" 
        size="small" 
        :loading="loading"
        :disabled="disabled"
        @click="$emit('start', instance.id)"
      >
        启动
      </el-button>
      <el-button 
        v-else-if="instance.observed_state === 'ready' || instance.observed_state === 'booting'" 
        type="danger" 
        size="small" 
        :loading="loading"
        :disabled="disabled"
        @click="$emit('stop', instance.id)"
      >
        停止
      </el-button>

      <el-button 
        v-if="instance.observed_state === 'ready'"
        :type="instance.has_stream ? 'info' : 'primary'"
        size="small"
        :disabled="disabled"
        @click="$emit('toggle-stream', instance.id)"
      >
        {{ instance.has_stream ? '关闭投屏' : '打开投屏' }}
      </el-button>
      <el-button
        v-if="instance.observed_state === 'ready'"
        type="success"
        size="small"
        :disabled="disabled"
        @click="$emit('good-doctor', instance.id)"
      >
        好医生
      </el-button>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { InstanceItem } from '../types'

const props = defineProps<{
  instance: InstanceItem
  selected: boolean
  loading?: boolean
  disabled?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:selected', val: boolean): void
  (e: 'start', id: string): void
  (e: 'stop', id: string): void
  (e: 'toggle-stream', id: string): void
  (e: 'good-doctor', id: string): void
}>()

// 状态标签色彩映射
const statusTagType = computed(() => {
  switch (props.instance.observed_state) {
    case 'ready':
      return 'success'
    case 'booting':
    case 'starting':
      return 'warning'
    case 'stopping':
      return 'warning'
    case 'error':
      return 'danger'
    case 'unknown':
      return 'warning'
    default:
      return 'info'
  }
})

function onSelectionChange(val: boolean | string | number) {
  emit('update:selected', !!val)
}
</script>

<style scoped>
.instance-card {
  border-radius: 8px;
  transition: all 0.2s ease-in-out;
}

.instance-card.is-selected {
  border-color: #409eff;
  background-color: #f0f7ff;
}

.card-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.header-info {
  display: flex;
  align-items: center;
  gap: 8px;
}

.instance-name {
  font-weight: 600;
  color: #303133;
}

.card-body {
  font-size: 13px;
  color: #606266;
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-bottom: 14px;
}

.prop-item {
  display: flex;
  justify-content: space-between;
}

.label {
  color: #909399;
}

.card-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  border-top: 1px solid #f2f3f5;
  padding-top: 10px;
}
</style>
