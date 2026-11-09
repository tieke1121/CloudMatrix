import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import EmptyDeviceList from './EmptyDeviceList.vue'
import ElementPlus, { ElButton } from 'element-plus'

// 测试空设备列表状态与交互
describe('EmptyDeviceList.vue', () => {
  it('正确渲染未配置提示与标题', () => {
    const wrapper = mount(EmptyDeviceList, {
      global: {
        plugins: [ElementPlus]
      }
    })

    expect(wrapper.text()).toContain('CloudMatrix 实例管理平台')
    expect(wrapper.text()).toContain('暂无纳管的 Android 实例')
    expect(wrapper.text()).toContain('未连接模拟器')
  })

  it('点击刷新按钮触发 refresh 事件', async () => {
    const wrapper = mount(EmptyDeviceList, {
      global: {
        plugins: [ElementPlus]
      }
    })

    const button = wrapper.findComponent(ElButton)
    expect(button.exists()).toBe(true)
    await button.trigger('click')

    expect(wrapper.emitted()).toHaveProperty('refresh')
  })
})
