<template>
  <el-dialog :model-value="modelValue" title="好医生" width="520px" @update:model-value="emit('update:modelValue', $event)">
    <el-skeleton v-if="loading && !status" :rows="3" animated />
    <el-descriptions v-else-if="status" :column="1" border>
      <el-descriptions-item label="实例">{{ instanceName }}</el-descriptions-item>
      <el-descriptions-item label="安装状态">{{ status.installed ? '已安装' : '未安装' }}</el-descriptions-item>
      <el-descriptions-item label="版本">{{ versionText }}</el-descriptions-item>
      <el-descriptions-item label="登录状态">{{ loginStatusText }}</el-descriptions-item>
      <el-descriptions-item label="账号标记">{{ status.account_label || '未保存' }}</el-descriptions-item>
    </el-descriptions>
    <el-alert
      title="验证码由你在好医生内手动输入；CloudMatrix 不读取或保存验证码。"
      type="info"
      :closable="false"
      show-icon
    />
    <el-form v-if="status?.installed" label-position="top">
      <el-form-item label="手机号后 4 位（可选）">
        <el-input v-model="phoneLast4" maxlength="4" inputmode="numeric" placeholder="仅用于区分账号" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button :disabled="loading" @click="emit('refresh')">刷新</el-button>
      <el-button :disabled="loading || !status?.installed" @click="emit('stop')">停止应用</el-button>
      <el-button type="primary" :disabled="loading || !status?.installed" @click="emit('open')">打开并投屏</el-button>
      <el-button
        type="warning"
        :disabled="loading || !status?.installed"
        @click="emit('update-login', 'verification_pending', phoneLast4)"
      >等待验证码</el-button>
      <el-button
        type="success"
        :disabled="loading || !status?.installed"
        @click="emit('update-login', 'logged_in', phoneLast4)"
      >标记已登录</el-button>
      <el-button
        type="danger"
        :disabled="loading || !status?.installed"
        @click="emit('update-login', 'expired', phoneLast4)"
      >登录已失效</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { GoodDoctorStatus, LoginStatus } from '../types'

const props = defineProps<{
  modelValue: boolean
  instanceName: string
  status?: GoodDoctorStatus
  loading?: boolean
}>()

const emit = defineEmits<{
  (event: 'update:modelValue', value: boolean): void
  (event: 'refresh' | 'open' | 'stop'): void
  (event: 'update-login', status: LoginStatus, phoneLast4: string): void
}>()

const phoneLast4 = ref('')
watch(() => props.status?.account_label, label => {
  phoneLast4.value = label?.startsWith('****') ? label.slice(4) : ''
}, { immediate: true })

const versionText = computed(() => {
  if (!props.status?.installed) return '-'
  return props.status.version_name
    ? `${props.status.version_name} (${props.status.version_code})`
    : props.status.version_code
})

const loginLabels: Record<LoginStatus, string> = {
  unknown: '未知',
  not_installed: '未安装',
  not_logged_in: '未登录',
  verification_pending: '等待验证码',
  logged_in: '已登录',
  expired: '登录已失效',
}
const loginStatusText = computed(() => props.status ? loginLabels[props.status.login_status] : '-')
</script>

<style scoped>
.el-alert,
.el-form {
  margin-top: 16px;
}
</style>
