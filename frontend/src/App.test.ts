import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus, { ElButton } from 'element-plus'
import { afterEach, describe, expect, it, vi } from 'vitest'
import App from './App.vue'
import InstanceCard from './components/InstanceCard.vue'
import type { InstanceItem } from './types'

const instance: InstanceItem = {
  id: 'test-id', name: 'Test instance', host_id: 'local', provider_type: 'mumu',
  provider_instance_id: '0', desired_state: 'stopped', observed_state: 'stopped',
  adb_serial: '', has_stream: false, created_at: '', updated_at: '',
}

function mockDesktop() {
  const api = {
    RefreshInstances: vi.fn().mockResolvedValue([instance]),
    ListInstances: vi.fn().mockResolvedValue([{ ...instance, observed_state: 'ready' }]),
    StartInstance: vi.fn().mockResolvedValue({ success: true, message: '启动成功' }),
    StopInstance: vi.fn(), OpenStream: vi.fn(), CloseStream: vi.fn(),
    InstallAPK: vi.fn(), GetTask: vi.fn(), CancelTask: vi.fn(),
    GetGoodDoctorStatus: vi.fn(), OpenGoodDoctor: vi.fn(), StopGoodDoctor: vi.fn(),
    UpdateGoodDoctorLoginStatus: vi.fn(),
  }
  window.go = { main: { App: api } }
  return api
}

afterEach(() => { delete window.go })

describe('desktop operations', () => {
  it('forwards a selected instance to batch start and reads back actual state', async () => {
    const api = mockDesktop()
    const wrapper = mount(App, { global: { plugins: [ElementPlus] } })
    try {
      await flushPromises()
      const card = wrapper.getComponent(InstanceCard)
      card.vm.$emit('update:selected', true)
      await wrapper.vm.$nextTick()
      const batch = wrapper.findAllComponents(ElButton).find(button => button.text().includes('批量启动'))
      expect(batch).toBeDefined()
      await batch!.trigger('click')
      await flushPromises()
      expect(api.StartInstance).toHaveBeenCalledOnce()
      expect(api.StartInstance).toHaveBeenCalledWith('test-id')
      expect(api.ListInstances).toHaveBeenCalledOnce()
      expect(card.props('instance').observed_state).toBe('ready')
    } finally {
      wrapper.unmount()
    }
  })

  it('shows discovery errors and does not show an empty-list success state', async () => {
    const api = mockDesktop()
    api.RefreshInstances.mockRejectedValue(new Error('缺少已验证的 CLI'))
    const wrapper = mount(App, { global: { plugins: [ElementPlus] } })
    try {
      await flushPromises()
      expect(wrapper.text()).toContain('缺少已验证的 CLI')
      expect(wrapper.findComponent(InstanceCard).exists()).toBe(false)
    } finally {
      wrapper.unmount()
    }
  })
})
