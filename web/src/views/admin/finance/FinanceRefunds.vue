<script setup lang="ts">
/** 财务中心·退款售后:发起退款(已支付订单)+ 用户申请审核。 */
import { ref, onMounted } from 'vue'
import { ElMessageBox } from 'element-plus'
import { financeApi } from '@/api'
import { fen2yuan, formatTime, REFUND_STATUS_NAME, REFUND_STATUS_TAG } from '@/utils/format'
import { toast } from '@/utils/notify'
import type { RefundOrder } from '@/types'

const tabs: { key: string; label: string }[] = [
  { key: '', label: '全部' },
  { key: 'pending', label: '待处理' },
  { key: 'refunded', label: '已退款' },
  { key: 'rejected', label: '已拒绝' },
]

const activeTab = ref('')
const list = ref<RefundOrder[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 10
const loading = ref(false)
const acting = ref('')

// 发起退款弹窗
const dialogVisible = ref(false)
const saving = ref(false)
const refundForm = ref({ order_no: '', reason: '' })

onMounted(load)

async function load() {
  loading.value = true
  try {
    const data = await financeApi.listRefunds({
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

function openCreate() {
  refundForm.value = { order_no: '', reason: '' }
  dialogVisible.value = true
}

/** 管理员直接退款:后端事务内条件更新(paid→refunded),重复/状态异常会报错。 */
async function submitRefund() {
  const f = refundForm.value
  if (!f.order_no.trim()) return toast('请输入订单号', 'warning')
  if (f.reason.trim().length < 2) return toast('请填写退款原因(至少 2 字)', 'warning')
  saving.value = true
  try {
    const r = await financeApi.adminRefund({ order_no: f.order_no.trim(), reason: f.reason.trim() })
    toast(`退款成功:${r.refund_no},¥${fen2yuan(r.amount)} 已原路退回(模拟)`)
    dialogVisible.value = false
    load()
  } finally {
    saving.value = false
  }
}

/** 处理用户申请:同意执行退款事务,拒绝仅标记。 */
async function process(r: RefundOrder, approve: boolean) {
  const verb = approve ? '同意退款' : '拒绝申请'
  await ElMessageBox.confirm(
    `确认${verb}?订单 ${r.order_no},金额 ¥${fen2yuan(r.amount)}${approve ? '(将执行退款并回补库存)' : '(订单保持已支付)'}`,
    `${verb}确认`,
    { type: approve ? 'warning' : 'info' },
  )
  acting.value = r.refund_no
  try {
    await financeApi.processRefund(r.refund_no, approve)
    toast(approve ? '已同意退款' : '已拒绝申请')
    load()
  } finally {
    acting.value = ''
  }
}
</script>

<template>
  <div>
    <div class="admin-head">
      <h2>退款售后</h2>
      <el-button type="primary" @click="openCreate">+ 发起退款</el-button>
    </div>

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
      <el-table-column label="退款单号" width="230">
        <template #default="{ row }"><span class="mono">{{ row.refund_no }}</span></template>
      </el-table-column>
      <el-table-column label="订单号" width="230">
        <template #default="{ row }"><span class="mono">{{ row.order_no }}</span></template>
      </el-table-column>
      <el-table-column label="用户 ID" prop="user_id" width="80" />
      <el-table-column label="金额" width="110">
        <template #default="{ row }">
          <span class="price">¥{{ fen2yuan(row.amount) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="原因" min-width="150">
        <template #default="{ row }">{{ row.reason || '-' }}</template>
      </el-table-column>
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="REFUND_STATUS_TAG[row.status]" size="small">{{ REFUND_STATUS_NAME[row.status] }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="申请时间" width="170">
        <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="处理时间" width="170">
        <template #default="{ row }">{{ row.processed_at ? formatTime(row.processed_at) : '-' }}</template>
      </el-table-column>
      <el-table-column label="操作" width="150" fixed="right">
        <template #default="{ row }">
          <template v-if="row.status === 'pending'">
            <el-button size="small" type="warning" :loading="acting === row.refund_no" @click="process(row, true)">
              同意
            </el-button>
            <el-button size="small" :loading="acting === row.refund_no" @click="process(row, false)">拒绝</el-button>
          </template>
          <span v-else class="no-op">-</span>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager" v-if="total > pageSize">
      <el-pagination layout="prev, pager, next" :total="total" :page-size="pageSize" :current-page="page" @current-change="onPageChange" />
    </div>

    <!-- 发起退款弹窗 -->
    <el-dialog v-model="dialogVisible" title="发起退款" width="480px" destroy-on-close>
      <el-form label-width="80px">
        <el-form-item label="订单号" required>
          <el-input v-model="refundForm.order_no" placeholder="FK 开头的已支付订单号" maxlength="32" />
        </el-form-item>
        <el-form-item label="退款原因" required>
          <el-input
            v-model="refundForm.reason"
            type="textarea"
            :rows="3"
            maxlength="200"
            placeholder="如:用户申请尺码不合适(2-200 字)"
          />
        </el-form-item>
      </el-form>
      <p class="tip">仅「已支付」订单可退款;整单全额原路退回(模拟),库存同步回补。</p>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submitRefund">确认退款</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.admin-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
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

.no-op {
  color: var(--fawn-text-sub);
}

.tip {
  margin: 0;
  font-size: 12.5px;
  color: var(--fawn-text-sub);
}

.pager {
  display: flex;
  justify-content: center;
  margin-top: 16px;
}
</style>
