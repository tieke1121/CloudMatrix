import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import InstanceCard from './InstanceCard.vue'
import ElementPlus, { ElButton } from 'element-plus'
import type { InstanceItem } from '../types'

describe('InstanceCard.vue', () => {
  const mockInstance: InstanceItem = {
    id: 'inst-1',
    host_id: 'local',
    provider_type: 'mumu',
    provider_instance_id: '0',
    name: 'MuMu-Test-0',
    desired_state: 'ready',
    observed_state: 'ready',
    adb_serial: '127.0.0.1:16384',
    has_stream: false,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString()
  }

  it('正确渲染实例信息与状态标签', () => {
    const wrapper = mount(InstanceCard, {
      props: {
        instance: mockInstance,
        selected: false
      },
      global: {
        plugins: [ElementPlus]
      }
    })

    expect(wrapper.text()).toContain('MuMu-Test-0')
    expect(wrapper.text()).toContain('127.0.0.1:16384')
    expect(wrapper.text()).toContain('ready')
  })

  it('处于 ready 状态时可触发停止与投屏事件', async () => {
    const wrapper = mount(InstanceCard, {
      props: {
        instance: mockInstance,
        selected: false
      },
      global: {
        plugins: [ElementPlus]
      }
    })

    const buttons = wrapper.findAllComponents(ElButton)
    expect(buttons.length).toBeGreaterThanOrEqual(3)

    // 点击停止
    await buttons[0].trigger('click')
    expect(wrapper.emitted()).toHaveProperty('stop')

    // 点击打开投屏
    await buttons[1].trigger('click')
    expect(wrapper.emitted()).toHaveProperty('toggle-stream')

    await buttons[2].trigger('click')
    expect(wrapper.emitted()).toHaveProperty('good-doctor')
  })
})
