// 实例状态枚举
export type InstanceState = 
  | 'discovered'
  | 'stopped'
  | 'starting'
  | 'booting'
  | 'ready'
  | 'stopping'
  | 'error'
  | 'unknown'

// 实例数据接口
export interface InstanceItem {
  id: string
  host_id: string
  provider_type: string
  provider_instance_id: string
  name: string
  desired_state: InstanceState
  observed_state: InstanceState
  adb_serial: string
  has_stream: boolean
  created_at: string
  updated_at: string
}

// 操作响应接口
export interface OperationResponse {
  success: boolean
  message: string
  task_id?: string
}

export type TaskStatus = 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelling' | 'cancelled' | 'interrupted'

export interface InstallTask {
  id: string
  status: TaskStatus
  targets: { id: string; instance_id: string; status: TaskStatus; error_message?: string }[]
}

export type LoginStatus =
  | 'unknown'
  | 'not_installed'
  | 'not_logged_in'
  | 'verification_pending'
  | 'logged_in'
  | 'expired'

export interface GoodDoctorStatus {
  instance_id: string
  installed: boolean
  package_name: string
  version_code: string
  version_name: string
  login_status: LoginStatus
  account_label: string
  updated_at: string
}

// 视频流画质等级
export type QualityProfile = 'full' | 'wall_preview' | 'throttled' | 'paused'

// 视频流状态信息
export interface VideoStreamStatus {
  instance_id: string
  profile: QualityProfile
  current_fps: number
  target_fps: number
  total_packets: number
  dropped_packets: number
  is_throttled: boolean
  is_connected: boolean
}

