import { ElAlert } from 'element-plus/es/components/alert/index.mjs'
import { ElAvatar } from 'element-plus/es/components/avatar/index.mjs'
import { ElButton } from 'element-plus/es/components/button/index.mjs'
import { ElEmpty } from 'element-plus/es/components/empty/index.mjs'
import { ElForm, ElFormItem } from 'element-plus/es/components/form/index.mjs'
import { ElIcon } from 'element-plus/es/components/icon/index.mjs'
import { ElInput } from 'element-plus/es/components/input/index.mjs'
import { ElInputNumber } from 'element-plus/es/components/input-number/index.mjs'
import { ElMenu, ElMenuItem, ElMenuItemGroup } from 'element-plus/es/components/menu/index.mjs'
import { ElOption, ElSelect } from 'element-plus/es/components/select/index.mjs'
import { ElPopconfirm } from 'element-plus/es/components/popconfirm/index.mjs'
import { ElTable, ElTableColumn } from 'element-plus/es/components/table/index.mjs'
import { ElTag } from 'element-plus/es/components/tag/index.mjs'
import { ElSwitch } from 'element-plus/es/components/switch/index.mjs'
import 'element-plus/es/components/alert/style/css'
import 'element-plus/es/components/avatar/style/css'
import 'element-plus/es/components/button/style/css'
import 'element-plus/es/components/empty/style/css'
import 'element-plus/es/components/form/style/css'
import 'element-plus/es/components/icon/style/css'
import 'element-plus/es/components/input/style/css'
import 'element-plus/es/components/input-number/style/css'
import 'element-plus/es/components/menu/style/css'
import 'element-plus/es/components/option/style/css'
import 'element-plus/es/components/popconfirm/style/css'
import 'element-plus/es/components/select/style/css'
import 'element-plus/es/components/switch/style/css'
import 'element-plus/es/components/table/style/css'
import 'element-plus/es/components/tag/style/css'
import { createApp } from 'vue'
import App from './App.vue'
import { router } from './router'
import './styles/tokens.css'
import './style.css'

// Installing the router starts its initial navigation. Waiting for isReady()
// before installation leaves hash-history navigation pending and renders a
// blank page on a fresh visit.
const app = createApp(App)
for (const component of [ElAlert, ElAvatar, ElButton, ElEmpty, ElForm, ElFormItem, ElIcon, ElInput, ElInputNumber, ElMenu, ElMenuItem, ElMenuItemGroup, ElOption, ElPopconfirm, ElSelect, ElSwitch, ElTable, ElTableColumn, ElTag]) {
  app.component(component.name!, component)
}
app.use(router).mount('#app')
