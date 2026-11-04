import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import { createApp } from 'vue'
import App from './App.vue'
import { router } from './router'
import './styles/tokens.css'
import './style.css'

router.isReady().then(() => createApp(App).use(ElementPlus).use(router).mount('#app'))
