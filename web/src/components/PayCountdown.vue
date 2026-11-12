<script setup lang="ts">
/** 支付倒计时:服务端给剩余秒数,组件本地每秒递减渲染;
 * 归零时 emit('expired'),由调用方决定刷新列表/订单。 */
import { ref, onMounted, onUnmounted, watch } from 'vue'

const props = defineProps<{ seconds: number }>()
const emit = defineEmits<{ expired: [] }>()

const remain = ref(props.seconds)
let timer: ReturnType<typeof setInterval> | null = null

function start() {
  stop()
  remain.value = props.seconds
  if (remain.value <= 0) {
    emitExpired()
    return
  }
  timer = setInterval(() => {
    remain.value--
    if (remain.value <= 0) {
      stop()
      emitExpired()
    }
  }, 1000)
}

function stop() {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}

function emitExpired() {
  emit('expired')
}

// 服务端重新拉取(如刷新/支付后返回)时重置倒计时
watch(() => props.seconds, start)
onMounted(start)
onUnmounted(stop)

const mm = () => String(Math.floor(remain.value / 60)).padStart(2, '0')
const ss = () => String(remain.value % 60).padStart(2, '0')
</script>

<template>
  <span class="pay-countdown" :class="{ urgent: remain <= 60 }">
    {{ mm() }}:{{ ss() }}
  </span>
</template>

<style scoped>
.pay-countdown {
  font-family: monospace;
  font-variant-numeric: tabular-nums;
  color: #d9915f;
  font-weight: 600;
}

.pay-countdown.urgent {
  color: var(--el-color-danger);
}
</style>
