<script setup lang="ts">
/** 订单详情:状态、时间轴(P1)、商品明细、收货信息、操作(支付/取消/确认收货)。 */
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { orderApi } from '@/api'
import { fen2yuan, ORDER_STATUS_NAME, ORDER_STATUS_TAG, formatTime } from '@/utils/format'
import { toast } from '@/utils/notify'
import PayCountdown from '@/components/PayCountdown.vue'
import type { Order } from '@/types'

const route = useRoute()
const router = useRouter()

const order = ref<Order | null>(null)
const loading = ref(true)
const acting = ref(false)
let pollTimer: ReturnType<typeof setTimeout> | null = null

const orderNo = computed(() => String(route.params.orderNo))

onMounted(async () => {
  await load()
  // 若订单刚下单还在 MQ 处理中,轮询直到落库
  if (!order.value) pollTimer = setTimeout(pollUntilCreated, 1200)
})

async function pollUntilCreated() {
  await load()
  if (!order.value && pollTimer) {
    pollTimer = setTimeout(pollUntilCreated, 1200)
  }
}

async function load() {
  try {
    order.value = await orderApi.detail(orderNo.value)
  } catch {
    order.value = null
  } finally {
    loading.value = false
  }
}

/** 时间轴:按订单流转阶段生成。 */
const timeline = computed(() => {
  const o = order.value
  if (!o) return []
  const steps = [
    { label: '订单创建', time: o.created_at },
    { label: '支付成功', time: o.pay_time },
    { label: '商家发货', time: o.ship_time },
    { label: '交易完成', time: o.finish_time },
  ]
  if (o.status === 'canceled') {
    return [
      { label: '订单创建', time: o.created_at },
      { label: `订单取消(${o.cancel_reason || '已取消'})`, time: o.cancel_time },
    ]
  }
  return steps.filter(s => s.time)
})

async function pay() {
  acting.value = true
  try {
    order.value = await orderApi.pay(orderNo.value)
    toast('支付成功(模拟)')
  } finally {
    acting.value = false
  }
}

async function cancel() {
  acting.value = true
  try {
    order.value = await orderApi.cancel(orderNo.value)
    toast('订单已取消,库存已回补')
  } finally {
    acting.value = false
  }
}

async function confirm() {
  acting.value = true
  try {
    order.value = await orderApi.confirm(orderNo.value)
    toast('确认收货,交易完成')
  } finally {
    acting.value = false
  }
}

/** 倒计时归零:延迟队列取消有秒级滞后,稍等后拉取最新状态。 */
async function onExpired() {
  await load()
  if (order.value?.status === 'pending_payment') setTimeout(load, 3000)
}
</script>

<template>
  <div v-loading="loading" class="od">
    <template v-if="order">
      <!-- 状态头 -->
      <div class="card status-card" :class="`st-${order.status}`">
        <div class="st-left">
          <el-tag :type="ORDER_STATUS_TAG[order.status]" size="large" effect="dark" round>
            {{ ORDER_STATUS_NAME[order.status] }}
          </el-tag>
          <p class="st-reason" v-if="order.status === 'canceled'">{{ order.cancel_reason }}</p>
          <p class="st-hint" v-else-if="order.status === 'pending_payment'">
            <template v-if="order.pay_remaining_seconds">
              剩余 <PayCountdown class="st-count" :seconds="order.pay_remaining_seconds" @expired="onExpired" />,
              超时订单将自动取消
            </template>
            <template v-else>支付已超时,订单取消处理中</template>
          </p>
        </div>
        <div class="st-amount">
          应付<span class="price big">¥{{ fen2yuan(order.pay_amount) }}</span>
        </div>
      </div>

      <!-- 时间轴 -->
      <div class="card" v-if="timeline.length">
        <h2 class="section-title">订单进度</h2>
        <el-timeline>
          <el-timeline-item
            v-for="(s, i) in timeline"
            :key="i"
            :timestamp="formatTime(s.time)"
            :type="i === timeline.length - 1 ? 'primary' : undefined"
            :hollow="i !== timeline.length - 1"
          >
            {{ s.label }}
          </el-timeline-item>
        </el-timeline>
      </div>

      <!-- 商品明细 -->
      <div class="card">
        <h2 class="section-title">商品明细</h2>
        <div v-for="(it, i) in order.items" :key="i" class="oi-row">
          <img class="oi-img" :src="it.image" :alt="it.product_name" @click="router.push(`/product/${it.product_id}`)" />
          <div class="oi-info">
            <p class="oi-name">{{ it.product_name }}</p>
            <span class="oi-size">尺码 {{ it.size }}</span>
          </div>
          <div class="oi-right">
            <span class="price">¥{{ fen2yuan(it.price) }}</span>
            <span class="oi-qty">× {{ it.quantity }}</span>
          </div>
        </div>
      </div>

      <!-- 收货信息 -->
      <div class="card">
        <h2 class="section-title">收货信息</h2>
        <el-descriptions :column="1" border size="default">
          <el-descriptions-item label="订单编号">{{ order.order_no }}</el-descriptions-item>
          <el-descriptions-item label="收货人">{{ order.receiver }}</el-descriptions-item>
          <el-descriptions-item label="联系电话">{{ order.phone }}</el-descriptions-item>
          <el-descriptions-item label="收货地址">{{ order.address }}</el-descriptions-item>
          <el-descriptions-item label="订单备注">{{ order.remark || '-' }}</el-descriptions-item>
          <el-descriptions-item label="创建时间">{{ formatTime(order.created_at) }}</el-descriptions-item>
        </el-descriptions>
      </div>

      <!-- 操作条 -->
      <div class="card act-bar">
        <el-button v-if="order.status === 'pending_payment'" @click="router.push('/orders')">返回列表</el-button>
        <el-button
          v-if="order.status === 'pending_payment'"
          type="danger"
          plain
          :loading="acting"
          @click="cancel"
        >
          取消订单
        </el-button>
        <el-button
          v-if="order.status === 'pending_payment'"
          type="primary"
          size="large"
          class="pay-btn"
          :loading="acting"
          @click="pay"
        >
          立即支付 ¥{{ fen2yuan(order.pay_amount) }}
        </el-button>
        <el-button
          v-if="order.status === 'shipped'"
          type="primary"
          :loading="acting"
          @click="confirm"
        >
          确认收货
        </el-button>
      </div>
    </template>

    <el-empty v-else-if="!loading" description="订单不存在,或正在处理中请稍后刷新">
      <el-button type="primary" @click="router.push('/orders')">回我的订单</el-button>
    </el-empty>
  </div>
</template>

<style scoped>
.od {
  max-width: 760px;
  margin: 0 auto;
}

.od .card {
  margin-bottom: 14px;
}

.status-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  background: linear-gradient(120deg, #fff 40%, var(--fawn-primary-bg));
}

.st-reason,
.st-hint {
  margin: 10px 0 0;
  font-size: 13px;
  color: var(--fawn-text-sub);
}

.st-amount {
  font-size: 13.5px;
  color: var(--fawn-text-sub);
  white-space: nowrap;
}

.price.big {
  font-size: 26px;
  margin-left: 6px;
}

.oi-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 0;
  border-bottom: 1px dashed var(--fawn-border);
}

.oi-row:last-child {
  border-bottom: none;
}

.oi-img {
  width: 64px;
  height: 64px;
  border-radius: 10px;
  object-fit: cover;
  cursor: pointer;
}

.oi-info {
  flex: 1;
}

.oi-name {
  margin: 0 0 6px;
  font-size: 14px;
  font-weight: 600;
}

.oi-size {
  font-size: 12.5px;
  color: var(--fawn-text-sub);
}

.oi-right {
  display: flex;
  align-items: baseline;
  gap: 10px;
}

.oi-qty {
  color: var(--fawn-text-sub);
  font-size: 13px;
}

.act-bar {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  flex-wrap: wrap;
}

.pay-btn {
  border-radius: 999px;
  min-width: 170px;
  font-weight: 600;
}
</style>
