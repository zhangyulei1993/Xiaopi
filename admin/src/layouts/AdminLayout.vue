<script setup lang="ts">
import AdminSidebar from '../components/AdminSidebar.vue'
import AdminTopbar from '../components/AdminTopbar.vue'

defineProps<{ active: string; title: string }>()
const emit = defineEmits<{ select: [value: string]; logout: [] }>()
</script>

<template>
  <main class="admin-shell">
    <AdminSidebar :active="active" @select="emit('select', $event)" />
    <section class="admin-main">
      <AdminTopbar :title="title" @logout="emit('logout')" />
      <div class="admin-content"><slot /></div>
    </section>
  </main>
</template>
