import { computed, onMounted, onUnmounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import type { GoodDoctorStatus, InstallTask, InstanceItem, LoginStatus, OperationResponse } from './types'

interface Desktop {
  RefreshInstances(): Promise<InstanceItem[]>
  ListInstances(): Promise<InstanceItem[]>
  StartInstance(id: string): Promise<OperationResponse>
  StopInstance(id: string): Promise<OperationResponse>
  OpenStream(id: string): Promise<OperationResponse>
  CloseStream(id: string): Promise<OperationResponse>
  InstallAPK(request: { apk_path: string; instance_ids: string[] }): Promise<OperationResponse>
  GetTask(id: string): Promise<InstallTask>
  CancelTask(id: string): Promise<void>
  GetGoodDoctorStatus(id: string): Promise<GoodDoctorStatus>
  OpenGoodDoctor(id: string): Promise<OperationResponse>
  StopGoodDoctor(id: string): Promise<OperationResponse>
  UpdateGoodDoctorLoginStatus(request: {
    instance_id: string
    status: LoginStatus
    phone_last4: string
  }): Promise<GoodDoctorStatus>
}

declare global {
  interface Window {
    go?: { main?: { App?: Desktop } }
  }
}

function desktop(): Desktop {
  const app = window.go?.main?.App
  if (!app) throw new Error('桌面服务未连接，请在 CloudMatrix 中打开')
  return app
}

const pending = new Set(['queued', 'running', 'cancelling'])

export function useDesktop() {
  const refreshing = ref(false)
  const busy = ref(false)
  const loadError = ref('')
  const instances = ref<InstanceItem[]>([])
  const selectedIds = ref<string[]>([])
  const taskId = ref('')
  const task = ref<InstallTask>()
  const taskError = ref('')
  const cancelling = ref(false)
  const goodDoctorLoading = ref(false)
  const goodDoctorStatus = ref<GoodDoctorStatus>()
  let goodDoctorRequestToken = 0
  const installPending = computed(() => !!taskId.value && (!task.value || pending.has(task.value.status)))
  let timer: ReturnType<typeof setTimeout> | undefined
  let disposed = false

  function updateInstances(items: InstanceItem[]) {
    instances.value = items ?? []
    const ids = new Set(instances.value.map(item => item.id))
    selectedIds.value = selectedIds.value.filter(id => ids.has(id))
  }

  async function fetchInstances() {
    if (refreshing.value || busy.value) return
    refreshing.value = true
    try {
      updateInstances(await desktop().RefreshInstances())
      loadError.value = ''
    } catch (err) {
      loadError.value = String(err)
    } finally {
      refreshing.value = false
    }
  }

  function onSelectChange(id: string, selected: boolean) {
    if (selected) {
      if (!selectedIds.value.includes(id)) selectedIds.value.push(id)
    } else {
      selectedIds.value = selectedIds.value.filter(item => item !== id)
    }
  }

  type Action = 'StartInstance' | 'StopInstance' | 'OpenStream' | 'CloseStream'
  async function runAction(action: Action, ids: string[]) {
    if (busy.value || refreshing.value || loadError.value || ids.length === 0) return
    busy.value = true
    let succeeded = 0
    try {
      const app = desktop()
      for (const id of ids) {
        try {
          const result = await app[action](id)
          if (!result.success) throw new Error(result.message)
          succeeded++
          if (ids.length === 1) ElMessage.success(result.message)
        } catch (err) {
          ElMessage.error(`${id}: ${String(err)}`)
        }
      }
      if (ids.length > 1) {
        ElMessage({ type: succeeded === ids.length ? 'success' : 'warning',
          message: `操作完成：成功 ${succeeded}，失败 ${ids.length - succeeded}` })
      }
      updateInstances(await app.ListInstances())
    } catch (err) {
      ElMessage.error(String(err))
    } finally {
      busy.value = false
    }
  }

  async function pollTask() {
    const id = taskId.value
    if (!id || disposed) return
    clearTimeout(timer)
    taskError.value = ''
    try {
      const result = await desktop().GetTask(id)
      if (disposed || id !== taskId.value) return
      task.value = result
      if (pending.has(result.status)) timer = setTimeout(pollTask, 700)
    } catch (err) {
      if (!disposed && id === taskId.value) taskError.value = String(err)
    }
  }

  async function submitInstall(apkPath: string): Promise<boolean> {
    if (busy.value || refreshing.value || loadError.value || installPending.value) return false
    busy.value = true
    try {
      const result = await desktop().InstallAPK({ apk_path: apkPath, instance_ids: [...selectedIds.value] })
      if (!result.success || !result.task_id) throw new Error(result.message)
      task.value = undefined
      taskId.value = result.task_id
      ElMessage.info(result.message)
      void pollTask()
      return true
    } catch (err) {
      ElMessage.error(String(err))
      return false
    } finally {
      busy.value = false
    }
  }

  async function cancelInstall() {
    if (!installPending.value || cancelling.value) return
    cancelling.value = true
    try {
      await desktop().CancelTask(taskId.value)
      ElMessage.info('已请求取消，等待实际结果')
      if (taskError.value) void pollTask()
    } catch (err) {
      ElMessage.error(String(err))
    } finally {
      cancelling.value = false
    }
  }

  async function loadGoodDoctor(id: string) {
    if (!id) return
    const token = ++goodDoctorRequestToken
    goodDoctorStatus.value = undefined
    goodDoctorLoading.value = true
    try {
      const result = await desktop().GetGoodDoctorStatus(id)
      if (token === goodDoctorRequestToken && result.instance_id === id) goodDoctorStatus.value = result
    } catch (err) {
      if (token === goodDoctorRequestToken) ElMessage.error(String(err))
    } finally {
      if (token === goodDoctorRequestToken) goodDoctorLoading.value = false
    }
  }

  async function runGoodDoctorAction(action: 'OpenGoodDoctor' | 'StopGoodDoctor', id: string) {
    if (goodDoctorLoading.value) return
    const token = ++goodDoctorRequestToken
    goodDoctorLoading.value = true
    try {
      const result = await desktop()[action](id)
      if (!result.success) throw new Error(result.message)
      if (token === goodDoctorRequestToken) ElMessage.success(result.message)
    } catch (err) {
      if (token === goodDoctorRequestToken) ElMessage.error(String(err))
    } finally {
      if (token === goodDoctorRequestToken) goodDoctorLoading.value = false
    }
  }

  async function updateGoodDoctorLogin(id: string, status: LoginStatus, phoneLast4: string) {
    if (goodDoctorLoading.value) return
    const token = ++goodDoctorRequestToken
    goodDoctorLoading.value = true
    try {
      const result = await desktop().UpdateGoodDoctorLoginStatus({
        instance_id: id,
        status,
        phone_last4: phoneLast4,
      })
      if (token === goodDoctorRequestToken && result.instance_id === id) {
        goodDoctorStatus.value = result
        ElMessage.success('登录状态已更新')
      }
    } catch (err) {
      if (token === goodDoctorRequestToken) ElMessage.error(String(err))
    } finally {
      if (token === goodDoctorRequestToken) goodDoctorLoading.value = false
    }
  }

  onMounted(fetchInstances)
  onUnmounted(() => { disposed = true; clearTimeout(timer) })

  return { refreshing, busy, loadError, instances, selectedIds, taskId, task, taskError,
    cancelling, installPending, goodDoctorLoading, goodDoctorStatus, fetchInstances, onSelectChange,
    runAction, pollTask, submitInstall, cancelInstall, loadGoodDoctor, runGoodDoctorAction, updateGoodDoctorLogin }
}
