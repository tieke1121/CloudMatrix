<template>
  <el-card 
    shadow="hover" 
    class="video-tile" 
    :class="{ 'is-selected': selected, 'is-throttled': streamStatus.is_throttled }"
  >
    <div class="tile-header">
      <div class="header-left">
        <el-checkbox 
          :model-value="selected" 
          @update:model-value="$emit('select', !selected)" 
        />
        <span class="tile-title">{{ instanceName }}</span>
      </div>
      <div class="header-right">
        <el-tag :type="statusTagType" size="small" effect="dark">
          {{ statusTagText }}
        </el-tag>
      </div>
    </div>

    <div class="tile-content">
      <!-- 视频画面画布容器 -->
      <div class="canvas-wrapper">
        <div v-if="!streamStatus.is_connected" class="offline-overlay">
          <el-icon class="icon-offline"><VideoCameraFilled /></el-icon>
          <span class="overlay-text">未连接视频源</span>
        </div>
        <div v-else-if="streamStatus.profile === 'paused'" class="paused-overlay">
          <span class="overlay-text">已暂停传输</span>
        </div>
        <div v-else class="stream-placeholder">
          <div class="screen-simulation">
            <span class="fps-indicator">
              {{ streamStatus.current_fps.toFixed(1) }} FPS / {{ streamStatus.target_fps }} FPS
            </span>
            <span v-if="streamStatus.is_throttled" class="throttled-badge">
              后台节能
            </span>
          </div>
        </div>
      </div>
    </div>

    <div class="tile-footer">
      <div class="footer-stats">
        <span class="stat-item">丢帧: {{ streamStatus.dropped_packets }}</span>
        <span class="stat-item">总帧: {{ streamStatus.total_packets }}</span>
      </div>
      <div class="footer-actions">
        <el-button 
          size="small" 
          type="primary" 
          plain 
          @click="$emit('focus', instanceId)"
        >
          原生聚焦
        </el-button>
        <el-button 
          v-if="streamStatus.is_connected"
          size="small" 
          :type="streamStatus.is_throttled ? 'warning' : 'info'" 
          plain 
          @click="toggleThrottled"
        >
          {{ streamStatus.is_throttled ? '恢复全速' : '后台节流' }}
        </el-button>
        <el-button 
          v-else 
          size="small" 
          type="success" 
          plain 
          @click="$emit('reconnect', instanceId)"
        >
          连接
        </el-button>
      </div>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { VideoCameraFilled } from '@element-plus/icons-vue'
import type { VideoStreamStatus, QualityProfile } from '../types'

const props = defineProps<{
  instanceId: string
  instanceName: string
  streamStatus: VideoStreamStatus
  selected?: boolean
}>()

const emit = defineEmits<{
  (e: 'select', val: boolean): void
  (e: 'focus', id: string): void
  (e: 'reconnect', id: string): void
  (e: 'update:profile', profile: QualityProfile): void
}>()

// 状态标签色彩计算
const statusTagType = computed(() => {
  if (!props.streamStatus.is_connected) {
    return 'info'
  }
  if (props.streamStatus.profile === 'paused') {
    return 'warning'
  }
  if (props.streamStatus.is_throttled) {
    return 'warning'
  }
  return 'success'
})

// 状态文本标签
const statusTagText = computed(() => {
  if (!props.streamStatus.is_connected) {
    return '离线'
  }
  if (props.streamStatus.profile === 'paused') {
    return '已暂停'
  }
  if (props.streamStatus.is_throttled) {
    return '节流 (3 FPS)'
  }
  return `在线 (${props.streamStatus.target_fps} FPS)`
})

// 切换后台节能节流状态
function toggleThrottled() {
  const newProfile = props.streamStatus.is_throttled ? 'wall_preview' : 'throttled'
  emit('update:profile', newProfile)
}
</script>

<style scoped>
.video-tile {
  border-radius: 8px;
  background-color: #ffffff;
  transition: all 0.2s ease-in-out;
  border: 1px solid #e4e7ed;
}

.video-tile.is-selected {
  border-color: #409eff;
  box-shadow: 0 0 0 2px rgba(64, 158, 255, 0.2);
}

.video-tile.is-throttled {
  opacity: 0.85;
}

.tile-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.tile-title {
  font-weight: 600;
  font-size: 14px;
  color: #303133;
}

.tile-content {
  position: relative;
  width: 100%;
  aspect-ratio: 9 / 16;
  background-color: #1a1a1a;
  border-radius: 4px;
  overflow: hidden;
  display: flex;
  justify-content: center;
  align-items: center;
}

.canvas-wrapper {
  width: 100%;
  height: 100%;
  position: relative;
}

.offline-overlay,
.paused-overlay {
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  width: 100%;
  height: 100%;
  color: #909399;
}

.icon-offline {
  font-size: 32px;
  margin-bottom: 8px;
}

.overlay-text {
  font-size: 12px;
}

.stream-placeholder {
  width: 100%;
  height: 100%;
  position: relative;
}

.screen-simulation {
  width: 100%;
  height: 100%;
  background: linear-gradient(180deg, #24292e 0%, #1e1e1e 100%);
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  padding: 8px;
  box-sizing: border-box;
}

.fps-indicator {
  font-size: 11px;
  color: #67c23a;
  font-family: monospace;
  background: rgba(0, 0, 0, 0.5);
  padding: 2px 6px;
  border-radius: 4px;
  align-self: flex-start;
}

.throttled-badge {
  font-size: 11px;
  color: #e6a23c;
  background: rgba(0, 0, 0, 0.6);
  padding: 2px 6px;
  border-radius: 4px;
  align-self: flex-end;
}

.tile-footer {
  margin-top: 8px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.footer-stats {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  color: #909399;
}

.footer-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
