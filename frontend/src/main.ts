import { createApp } from 'vue'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import './style.css'
import App from './App.vue'

// 初始化应用并挂载 Element Plus
const app = createApp(App)
app.use(ElementPlus)
app.mount('#app')
