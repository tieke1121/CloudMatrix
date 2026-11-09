import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { GoodDoctorStatus } from './types'
import { useDesktop } from './useDesktop'

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>(done => { resolve = done })
  return { promise, resolve }
}

afterEach(() => { delete window.go })

describe('useDesktop good doctor state', () => {
  it('ignores a stale response from the previously selected instance', async () => {
    const first = deferred<GoodDoctorStatus>()
    const second = deferred<GoodDoctorStatus>()
    const api = {
      RefreshInstances: vi.fn().mockResolvedValue([]),
      GetGoodDoctorStatus: vi.fn()
        .mockImplementationOnce(() => first.promise)
        .mockImplementationOnce(() => second.promise),
    }
    window.go = { main: { App: api as never } }
    const wrapper = mount(defineComponent({ setup: useDesktop, template: '<div />' }))
    await flushPromises()

    const vm = wrapper.vm as any
    const requestA = vm.loadGoodDoctor('inst-a')
    const requestB = vm.loadGoodDoctor('inst-b')
    second.resolve({
      instance_id: 'inst-b', installed: true, package_name: 'com.haoyisheng.mobile.app',
      version_code: '2', version_name: '2.0', login_status: 'not_logged_in', account_label: '****2222', updated_at: '',
    })
    await requestB
    first.resolve({
      instance_id: 'inst-a', installed: true, package_name: 'com.haoyisheng.mobile.app',
      version_code: '1', version_name: '1.0', login_status: 'logged_in', account_label: '****1111', updated_at: '',
    })
    await requestA

    expect(vm.goodDoctorStatus?.instance_id).toBe('inst-b')
    expect(vm.goodDoctorStatus?.account_label).toBe('****2222')
    wrapper.unmount()
  })
})
