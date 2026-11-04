import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import { createApp } from 'vue'
import App from './App.vue'
import { router } from './router'
import './styles/tokens.css'
import './style.css'

// Installing the router starts its initial navigation. Waiting for isReady()
// before installation leaves hash-history navigation pending and renders a
// blank page on a fresh visit.
createApp(App).use(ElementPlus).use(router).mount('#app')
