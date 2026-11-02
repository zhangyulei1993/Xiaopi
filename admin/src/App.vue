<script setup lang="ts">
import { computed, ref } from 'vue'
const apiBase = import.meta.env.VITE_API_BASE || 'http://127.0.0.1:18080/api'
const loggedIn = ref(false)
const tab = ref('房源审核')
const users = ref([{ name:'微信用户', phone:'未绑定', status:'正常' }])
const properties = ref<{title:string;community:string;status:string}[]>([])
const leads = ref<{sourceName:string;community:string;sourcePhone:string;status:string}[]>([])
const username = ref('admin'); const password = ref(''); const loginError = ref('')
const login = async () => { loginError.value=''; try { const auth = await fetch(`${apiBase}/v1/admin/login`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({username:username.value,password:password.value})}).then(r=>r.json()); if(auth.code!==0) throw new Error(auth.message); localStorage.setItem('xiaopi_admin_token',auth.data.accessToken); loggedIn.value = true; const [p,l] = await Promise.all([fetch(`${apiBase}/v1/properties`).then(r=>r.json()), fetch(`${apiBase}/v1/property-leads`).then(r=>r.json())]); properties.value = p.data; leads.value = l.data } catch(e) { loginError.value = e instanceof Error ? e.message : '登录失败' } }
const rows = computed(() => tab.value === '客户管理' ? users.value : tab.value === '线索管理' ? leads.value : properties.value)
</script>
<template>
  <main v-if="!loggedIn" class="login"><section><div class="logo">小皮租房</div><p>运营管理后台</p><form @submit.prevent="login"><input v-model="username" placeholder="管理员账号" /><input v-model="password" placeholder="管理员密码" type="password" /><div v-if="loginError" class="error">{{ loginError }}</div><button type="submit">登录后台</button></form></section></main>
  <main v-else class="layout"><aside><div class="brand">小皮租房</div><div class="nav" :class="{active:tab==='房源审核'}" @click="tab='房源审核'">房源审核</div><div class="nav" :class="{active:tab==='线索管理'}" @click="tab='线索管理'">线索管理</div><div class="nav" :class="{active:tab==='客户管理'}" @click="tab='客户管理'">客户管理</div><div class="nav">操作日志</div></aside><section class="content"><header><h1>{{ tab }}</h1><span>管理员</span></header><div class="stats"><div><b>{{ rows.length }}</b><small>{{ tab === '客户管理' ? '客户总数' : tab === '线索管理' ? '待处理线索' : '房源总数' }}</small></div><div><b>0</b><small>今日新增</small></div><div><b>0</b><small>待处理事项</small></div></div><div class="table"><div class="tr head"><span>名称</span><span>信息</span><span>状态</span><span>操作</span></div><div v-for="row in rows" :key="row.name || row.title || row.community" class="tr"><span>{{ row.name || row.title || row.sourceName }}</span><span>{{ row.phone || row.community || row.sourcePhone }}</span><span>{{ row.status }}</span><button>查看</button></div></div></section></main>
</template>
