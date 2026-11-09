<template>
  <div class="empty-device-container">
    <el-card shadow="never" class="status-card">
      <template #header>
        <div class="card-header">
          <span class="title">CloudMatrix 实例管理平台</span>
          <el-tag type="warning" effect="plain">未连接模拟器</el-tag>
        </div>
      </template>

      <el-empty
        description="暂无纳管的 Android 实例"
        :image-size="120"
      >
        <div class="tip-box">
          <p class="tip-title">系统状态检查提示：</p>
          <ul class="tip-list">
            <li>宿主未检测到已就绪的模拟器执行路径或可用实例</li>
            <li>请在配置中指定模拟器 CLI 路径（如 MuMuManager.exe）或连接已授权 ADB 设备</li>
            <li>平台已就绪，当前处于单宿主就绪等待状态</li>
          </ul>
          <div class="actions">
            <el-button type="primary" :loading="refreshing" @click="handleRefresh">
              刷新实例列表
            </el-button>
          </div>
        </div>
      </el-empty>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'

const refreshing = ref(false)

const emit = defineEmits<{
  (e: 'refresh'): void
}>()

// 触发刷新检查
function handleRefresh() {
  refreshing.value = true
  emit('refresh')
  setTimeout(() => {
    refreshing.value = false
  }, 500)
}
</script>

<style scoped>
.empty-device-container {
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 40px 20px;
}

.status-card {
  width: 100%;
  max-width: 680px;
  border-radius: 8px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.title {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
}

.tip-box {
  text-align: left;
  background-color: #fafafa;
  border: 1px solid #ebeef5;
  border-radius: 6px;
  padding: 16px 20px;
  margin-top: 12px;
}

.tip-title {
  font-weight: 600;
  font-size: 14px;
  color: #606266;
  margin: 0 0 8px 0;
}

.tip-list {
  margin: 0;
  padding-left: 20px;
  color: #909399;
  font-size: 13px;
  line-height: 1.8;
}

.actions {
  margin-top: 16px;
  text-align: center;
}
</style>
