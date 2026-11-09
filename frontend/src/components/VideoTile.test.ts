import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import VideoTile from './VideoTile.vue'
import ElementPlus, { ElButton } from 'element-plus'
import type { VideoStreamStatus } from '../types'

describe('VideoTile.vue', () => {
  const onlineStatus: VideoStreamStatus = {
    instance_id: 'vm-01',
    profile: 'wall_preview',
    current_fps: 11.8,
    target_fps: 12,
    total_packets: 120,
    dropped_packets: 4,
    is_throttled: false,
    is_connected: true
  }

  const offlineStatus: VideoStreamStatus = {
    instance_id: 'vm-02',
    profile: 'wall_preview',
    current_fps: 0,
    target_fps: 12,
    total_packets: 0,
    dropped_packets: 0,
    is_throttled: false,
    is_connected: false
  }

  it('未连接状态下正确显示离线提示并提供连接按钮', async () => {
    const wrapper = mount(VideoTile, {
      props: {
        instanceId: 'vm-02',
        instanceName: '模拟器-02',
        streamStatus: offlineStatus,
        selected: false
      },
      global: {
        plugins: [ElementPlus]
      }
    })

    expect(wrapper.text()).toContain('模拟器-02')
    expect(wrapper.text()).toContain('离线')
    expect(wrapper.text()).toContain('未连接视频源')

    // 验证连接按钮点击触发 reconnect 事件
    const buttons = wrapper.findAllComponents(ElButton)
    const connectBtn = buttons.find(b => b.text().includes('连接'))
    expect(connectBtn).toBeDefined()
    await connectBtn!.trigger('click')
    expect(wrapper.emitted()).toHaveProperty('reconnect')
    expect(wrapper.emitted('reconnect')![0]).toEqual(['vm-02'])
  })

  it('在线状态下正确显示帧率指标与丢帧统计', () => {
    const wrapper = mount(VideoTile, {
      props: {
        instanceId: 'vm-01',
        instanceName: '模拟器-01',
        streamStatus: onlineStatus,
        selected: true
      },
      global: {
        plugins: [ElementPlus]
      }
    })

    expect(wrapper.text()).toContain('11.8 FPS')
    expect(wrapper.text()).toContain('丢帧: 4')
    expect(wrapper.text()).toContain('总帧: 120')
  })

  it('点击原生聚焦与后台节流按钮触发对应事件', async () => {
    const wrapper = mount(VideoTile, {
      props: {
        instanceId: 'vm-01',
        instanceName: '模拟器-01',
        streamStatus: onlineStatus,
        selected: false
      },
      global: {
        plugins: [ElementPlus]
      }
    })

    const buttons = wrapper.findAllComponents(ElButton)
    const focusBtn = buttons.find(b => b.text().includes('原生聚焦'))
    expect(focusBtn).toBeDefined()
    await focusBtn!.trigger('click')
    expect(wrapper.emitted()).toHaveProperty('focus')
    expect(wrapper.emitted('focus')![0]).toEqual(['vm-01'])

    // 点击后台节流
    const throttleBtn = buttons.find(b => b.text().includes('后台节流'))
    expect(throttleBtn).toBeDefined()
    await throttleBtn!.trigger('click')
    expect(wrapper.emitted()).toHaveProperty('update:profile')
    expect(wrapper.emitted('update:profile')![0]).toEqual(['throttled'])
  })
})
