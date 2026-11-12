<script setup lang="ts">
/** 我的订单:状态 Tab + 分页列表(待支付展示剩余支付倒计时)。 */
import { ref, onMounted } from 'vue'
import { orderApi } from '@/api'
import { fen2yuan, ORDER_STATUS_NAME, ORDER_STATUS_TAG, formatTime } from '@/utils/format'
import PayCountdown from '@/components/PayCountdown.vue'
import type { Order, OrderStatus } from '@/types'

const tabs: { key: string; label: string }[] = [
  { key: '', label: '全部' },
  { key: 'pending_payment', label: '待支付' },
  { key: 'paid', label: '已支付' },
  { key: 'shipped', label: '已发货' },
  { key: 'completed', label: '已完成' },
  { key: 'refunded', label: '已退款' },
  { key: 'canceled', label: '已取消' },
]

const activeTab = ref('')
const list = ref<Order[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 10
const loading = ref(false)

onMounted(load)

async function load() {
  loading.value = true
  try {
    const data = await orderApi.list({
      page: page.value,
      page_size: pageSize,
      status: activeTab.value || undefined,
    })
    list.value = data.list
    total.value = data.total
  } finally {
    loading.value = false
  }
}

function switchTab(key: string) {
  activeTab.value = key
  page.value = 1
  load()
}

function onPageChange(p: number) {
  page.value = p
  load()
}

function firstItemImages(order: Order): string[] {
  return (order.items ?? []).slice(0, 4).map(i => i.image)
}

function itemCount(order: Order): number {
  return (order.items ?? []).reduce((n, i) => n + i.quantity, 0)
}
</script>

<template>
  <div>
    <h1 class="page-title">我的订单</h1>

    <div class="tabs">
      <button
        v-for="t in tabs"
        :key="t.key"
        class="tab-btn"
        :class="{ active: activeTab === t.key }"
        @click="switchTab(t.key)"
      >
        {{ t.label }}
      </button>
    </div>

    <div v-loading="loading" class="list">
      <template v-if="list.length">
        <RouterLink
          v-for="o in list"
          :key="o.order_no"
          :to="`/orders/${o.order_no}`"
          class="card order-card"
        >
          <div class="oc-head">
            <span class="oc-no">{{ o.order_no }}</span>
            <el-tag :type="ORDER_STATUS_TAG[o.status as OrderStatus]" effect="light" round>
              {{ ORDER_STATUS_NAME[o.status as OrderStatus] }}
            </el-tag>
          </div>
          <div class="oc-body">
            <div class="oc-thumbs">
              <img v-for="(img, i) in firstItemImages(o)" :key="i" :src="img" class="oc-thumb" />
            </div>
            <div class="oc-info">
              <p class="oc-time">{{ formatTime(o.created_at) }}</p>
              <p class="oc-count">共 {{ itemCount(o) }} 件</p>
            </div>
            <div class="oc-amount">
              <span class="price">¥{{ fen2yuan(o.pay_amount) }}</span>
              <span v-if="o.status === 'pending_payment' && o.pay_remaining_seconds" class="oc-pay-hint">
                <PayCountdown :seconds="o.pay_remaining_seconds" @expired="load" /> 后自动取消
              </span>
              <span v-else-if="o.status === 'pending_payment'" class="oc-pay-hint">支付超时处理中</span>
            </div>
          </div>
        </RouterLink>
      </template>
      <el-empty v-else-if="!loading" description="暂无相关订单">
        <RouterLink to="/"><el-button type="primary">去逛逛</el-button></RouterLink>
      </el-empty>
    </div>

    <div class="pager" v-if="total > pageSize">
      <el-pagination
        layout="prev, pager, next"
        :total="total"
        :page-size="pageSize"
        :current-page="page"
        @current-change="onPageChange"
      />
    </div>
  </div>
</template>

<style scoped>
.page-title {
  font-size: 20px;
  margin: 4px 0 14px;
}

.tabs {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 16px;
}

.tab-btn {
  border: 1px solid var(--fawn-border);
  background: #fff;
  color: var(--fawn-text-sub);
  border-radius: 999px;
  padding: 6px 16px;
  font-size: 13.5px;
  cursor: pointer;
  transition: all 0.15s;
}

.tab-btn.active {
  background: var(--fawn-primary);
  border-color: var(--fawn-primary);
  color: #fff;
  font-weight: 600;
}

.order-card {
  display: block;
  text-decoration: none;
  color: inherit;
  margin-bottom: 12px;
  transition: box-shadow 0.2s;
}

.order-card:hover {
  box-shadow: 0 8px 20px rgba(232, 115, 74, 0.12);
}

.oc-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-bottom: 10px;
  border-bottom: 1px solid var(--fawn-border);
}

.oc-no {
  font-size: 13px;
  color: var(--fawn-text-sub);
  font-family: monospace;
}

.oc-body {
  display: flex;
  align-items: center;
  gap: 14px;
  padding-top: 12px;
}

.oc-thumbs {
  display: flex;
  gap: 6px;
}

.oc-thumb {
  width: 52px;
  height: 52px;
  border-radius: 8px;
  object-fit: cover;
}

.oc-info {
  flex: 1;
}

.oc-time,
.oc-count {
  margin: 2px 0;
  font-size: 12.5px;
  color: var(--fawn-text-sub);
}

.oc-amount {
  text-align: right;
}

.oc-amount .price {
  font-size: 17px;
}

.oc-pay-hint {
  display: block;
  margin-top: 4px;
  font-size: 11.5px;
  color: #d9915f;
}

.list {
  min-height: 180px;
}

.pager {
  display: flex;
  justify-content: center;
  margin-top: 18px;
}
</style>
