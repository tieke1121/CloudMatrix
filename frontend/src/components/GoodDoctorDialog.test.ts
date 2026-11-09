import { mount } from '@vue/test-utils'
import ElementPlus, { ElButton } from 'element-plus'
import { describe, expect, it } from 'vitest'
import GoodDoctorDialog from './GoodDoctorDialog.vue'

describe('GoodDoctorDialog', () => {
  it('keeps verification-code entry inside the app and emits only status plus phone suffix', async () => {
    const wrapper = mount(GoodDoctorDialog, {
      props: {
        modelValue: true,
        instanceName: 'MuMu-0',
        status: {
          instance_id: 'inst-1', installed: true, package_name: 'com.haoyisheng.mobile.app',
          version_code: '60437', version_name: '6.4.37', login_status: 'not_logged_in',
          account_label: '', updated_at: '',
        },
      },
      global: {
        plugins: [ElementPlus],
        stubs: { ElDialog: { template: '<div><slot /><slot name="footer" /></div>' } },
      },
    })
    expect(wrapper.text()).toContain('CloudMatrix 不读取或保存验证码')
    const loginButton = wrapper.findAllComponents(ElButton).find(button => button.text() === '标记已登录')
    await loginButton!.trigger('click')
    expect(wrapper.emitted('update-login')?.[0]).toEqual(['logged_in', ''])
  })
})
