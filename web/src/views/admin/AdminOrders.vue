<script setup lang="ts">
/** 订单管理:全量订单列表(状态筛选)+ 发货/退款操作。 */
import { ref, onMounted } from 'vue'
import { ElMessageBox } from 'element-plus'
import { adminApi, financeApi } from '@/api'
import { fen2yuan, ORDER_STATUS_NAME, ORDER_STATUS_TAG, formatTime } from '@/utils/format'
import { toast } from '@/utils/notify'
import type { Order, OrderStatus } from '@/types'

const tabs: { key: string; label: string }[] = [
  { key: '', label: '全部' },
  { key: 'pending_payment', label: '待支付' },
  { key: 'paid', label: '待发货' },
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
const shipping = ref('')
const refunding = ref('')

onMounted(load)

async function load() {
  loading.value = true
  try {
    const data = await adminApi.listOrders({
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

async function ship(o: Order) {
  shipping.value = o.order_no
  try {
    await adminApi.ship(o.order_no)
    toast(`订单 ${o.order_no} 已发货`)
    load()
  } finally {
    shipping.value = ''
  }
}

/** 对已支付订单退款:后端事务内条件更新(paid→refunded),重复/状态异常返回冲突错误。 */
async function refund(o: Order) {
  const { value } = await ElMessageBox.prompt(
    `订单 ${o.order_no},金额 ¥${fen2yuan(o.pay_amount)}。整单全额退款(模拟),库存回补。`,
    '退款原因',
    {
      confirmButtonText: '确认退款',
      cancelButtonText: '取消',
      inputPlaceholder: '请输入退款原因(2-200 字)',
      inputValidator: (v: string) => (v && v.trim().length >= 2 ? true : '退款原因至少 2 字'),
      type: 'warning',
    },
  )
  refunding.value = o.order_no
  try {
    const r = await financeApi.adminRefund({ order_no: o.order_no, reason: value.trim() })
    toast(`退款成功:${r.refund_no}`)
    load()
  } finally {
    refunding.value = ''
  }
}

function onPageChange(p: number) {
  page.value = p
  load()
}
</script>

<template>
  <div>
    <div class="admin-head"><h2>订单管理</h2></div>

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

    <el-table :data="list" v-loading="loading" stripe class="table">
      <el-table-column label="订单号" width="220">
        <template #default="{ row }">
          <span class="mono">{{ row.order_no }}</span>
        </template>
      </el-table-column>
      <el-table-column label="收货信息" min-width="200">
        <template #default="{ row }">
          <div class="addr-cell">
            <span>{{ row.receiver }} · {{ row.phone }}</span>
            <span class="addr">{{ row.address }}</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="金额" width="100">
        <template #default="{ row }">
          <span class="price">¥{{ fen2yuan(row.pay_amount) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="ORDER_STATUS_TAG[row.status as OrderStatus]" size="small">
            {{ ORDER_STATUS_NAME[row.status as OrderStatus] }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="下单时间" width="170">
        <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="160" fixed="right">
        <template #default="{ row }">
          <template v-if="row.status === 'paid'">
            <el-button size="small" type="primary" :loading="shipping === row.order_no" @click="ship(row)">
              发货
            </el-button>
            <el-button size="small" type="warning" :loading="refunding === row.order_no" @click="refund(row)">
              退款
            </el-button>
          </template>
          <span v-else class="no-op">-</span>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager" v-if="total > pageSize">
      <el-pagination layout="prev, pager, next" :total="total" :page-size="pageSize" :current-page="page" @current-change="onPageChange" />
    </div>
  </div>
</template>

<style scoped>
.admin-head {
  margin-bottom: 14px;
}

.admin-head h2 {
  margin: 0;
}

.tabs {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}

.tab-btn {
  border: 1px solid var(--fawn-border);
  background: #fff;
  color: var(--fawn-text-sub);
  border-radius: 999px;
  padding: 6px 14px;
  font-size: 13px;
  cursor: pointer;
}

.tab-btn.active {
  background: var(--fawn-primary);
  border-color: var(--fawn-primary);
  color: #fff;
  font-weight: 600;
}

.table {
  background: #fff;
  border-radius: var(--fawn-radius);
}

.mono {
  font-family: monospace;
  font-size: 12.5px;
}

.addr-cell {
  display: flex;
  flex-direction: column;
  gap: 3px;
  font-size: 13px;
}

.addr {
  color: var(--fawn-text-sub);
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.no-op {
  color: var(--fawn-text-sub);
}

.pager {
  display: flex;
  justify-content: center;
  margin-top: 16px;
}
</style>
