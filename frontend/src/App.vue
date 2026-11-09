<template>
  <el-container class="app-layout">
    <el-header class="app-header">
      <div class="header-left">
        <h2 class="app-title">CloudMatrix</h2>
        <span class="app-subtitle">Windows Android 实例管理平台</span>
      </div>
      <div class="header-right">
        <el-tag size="small" type="success">Host: Local</el-tag>
      </div>
    </el-header>

    <el-main class="app-main">
      <div class="toolbar">
        <div class="toolbar-left">
          <el-button type="primary" :loading="refreshing" :disabled="busy" @click="fetchInstances">
            刷新实例
          </el-button>
          <el-button 
            type="success" 
            :disabled="actionsDisabled || selectedIds.length === 0"
            @click="runAction('StartInstance', [...selectedIds])"
          >
            批量启动 ({{ selectedIds.length }})
          </el-button>
          <el-button 
            type="danger" 
            :disabled="actionsDisabled || selectedIds.length === 0"
            @click="runAction('StopInstance', [...selectedIds])"
          >
            批量停止 ({{ selectedIds.length }})
          </el-button>
          <el-button 
            type="warning" 
            :disabled="actionsDisabled || installPending || selectedIds.length === 0"
            @click="dialogVisible = true"
          >
            安装好医生 APK
          </el-button>
        </div>
        <div class="toolbar-right">
          <span class="counter">纳管总数: {{ instances.length }}</span>
        </div>
      </div>

      <el-alert v-if="loadError" type="error" :title="loadError" :closable="false" show-icon />
      <el-skeleton v-else-if="refreshing && instances.length === 0" :rows="3" animated />
      <div v-if="!loadError && !refreshing && instances.length === 0" class="empty-wrap">
        <EmptyDeviceList @refresh="fetchInstances" />
      </div>

      <div v-if="instances.length" class="cards-grid">
        <InstanceCard
          v-for="inst in instances"
          :key="inst.id"
          :instance="inst"
          :selected="selectedIds.includes(inst.id)"
          :disabled="actionsDisabled"
          @update:selected="(val) => onSelectChange(inst.id, val)"
          @start="id => runAction('StartInstance', [id])"
          @stop="id => runAction('StopInstance', [id])"
          @toggle-stream="handleToggleStream"
          @good-doctor="openGoodDoctorDialog"
        />
      </div>

      <section v-if="taskId" aria-label="APK 分发结果">
        <p>APK 任务 {{ taskId }}：{{ task?.status ?? '正在查询' }}</p>
        <el-button v-if="installPending" :loading="cancelling" @click="cancelInstall">取消任务</el-button>
        <template v-if="taskError">
          <el-alert type="error" :title="taskError" :closable="false" />
          <el-button @click="pollTask">重新查询</el-button>
        </template>
        <ul v-if="task?.targets?.length">
          <li v-for="target in task.targets" :key="target.id">
            {{ target.instance_id }}：{{ target.status }} {{ target.error_message }}
          </li>
        </ul>
      </section>

      <GoodDoctorDialog
        v-model="goodDoctorVisible"
        :instance-name="goodDoctorInstance?.name ?? ''"
        :status="goodDoctorStatus"
        :loading="goodDoctorLoading"
        @refresh="refreshGoodDoctor"
        @open="handleGoodDoctorAction('OpenGoodDoctor')"
        @stop="handleGoodDoctorAction('StopGoodDoctor')"
        @update-login="handleGoodDoctorLogin"
      />

      <el-dialog v-model="dialogVisible" title="安装好医生 APK" width="500px">
        <el-form label-position="top">
          <el-form-item label="本地 APK 绝对路径">
            <el-input v-model="apkPath" placeholder="例如: D:\app-release.apk" />
          </el-form-item>
        </el-form>
        <template #footer>
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="busy" :disabled="actionsDisabled || !apkPath.trim()" @click="handleConfirmInstall">
            校验并安装
          </el-button>
        </template>
      </el-dialog>
    </el-main>
  </el-container>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import EmptyDeviceList from './components/EmptyDeviceList.vue'
import GoodDoctorDialog from './components/GoodDoctorDialog.vue'
import InstanceCard from './components/InstanceCard.vue'
import type { LoginStatus } from './types'
import { useDesktop } from './useDesktop'

const dialogVisible = ref(false)
const apkPath = ref('')
const goodDoctorVisible = ref(false)
const goodDoctorInstanceId = ref('')
const { refreshing, busy, loadError, instances, selectedIds, taskId, task, taskError,
  cancelling, installPending, goodDoctorLoading, goodDoctorStatus, fetchInstances, onSelectChange,
  runAction, pollTask, submitInstall, cancelInstall, loadGoodDoctor, runGoodDoctorAction,
  updateGoodDoctorLogin } = useDesktop()
const actionsDisabled = computed(() => busy.value || refreshing.value || installPending.value || !!loadError.value)
const goodDoctorInstance = computed(() => instances.value.find(item => item.id === goodDoctorInstanceId.value))

function handleToggleStream(id: string) {
  const instance = instances.value.find(item => item.id === id)
  if (instance) void runAction(instance.has_stream ? 'CloseStream' : 'OpenStream', [id])
}

function openGoodDoctorDialog(id: string) {
  goodDoctorInstanceId.value = id
  goodDoctorVisible.value = true
  void loadGoodDoctor(id)
}

function refreshGoodDoctor() {
  void loadGoodDoctor(goodDoctorInstanceId.value)
}

async function handleGoodDoctorAction(action: 'OpenGoodDoctor' | 'StopGoodDoctor') {
  await runGoodDoctorAction(action, goodDoctorInstanceId.value)
  await loadGoodDoctor(goodDoctorInstanceId.value)
}

function handleGoodDoctorLogin(status: LoginStatus, phoneLast4: string) {
  void updateGoodDoctorLogin(goodDoctorInstanceId.value, status, phoneLast4)
}

async function handleConfirmInstall() {
  if (await submitInstall(apkPath.value)) {
    dialogVisible.value = false
    apkPath.value = ''
  }
}
</script>

<style scoped>
.app-layout {
  min-height: 100vh;
}

.app-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  background-color: #ffffff;
  border-bottom: 1px solid #dcdfe6;
  padding: 0 24px;
  height: 60px;
}

.header-left {
  display: flex;
  align-items: baseline;
  gap: 12px;
}

.app-title {
  margin: 0;
  font-size: 20px;
  color: #303133;
}

.app-subtitle {
  font-size: 13px;
  color: #909399;
}

.app-main {
  background-color: #f5f7fa;
  padding: 24px;
}

.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  background-color: #ffffff;
  padding: 12px 20px;
  border-radius: 6px;
  margin-bottom: 20px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.counter {
  font-size: 13px;
  color: #909399;
}

.cards-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
}

.empty-wrap {
  margin-top: 20px;
}
</style>
