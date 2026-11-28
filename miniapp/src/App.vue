<script setup lang="ts">
import './styles/global.css'
import { onHide, onShow } from '@dcloudio/uni-app'
import { setUploadForeground } from './utils/foreground-upload'

onHide(() => setUploadForeground(false))
onShow(() => setUploadForeground(true))
</script>

<template><slot /></template>
