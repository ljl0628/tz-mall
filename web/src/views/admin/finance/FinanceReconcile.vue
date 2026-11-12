<script setup lang="ts">
/** 财务中心·对账管理:一键对账(订单总额 vs 流水总额)+ 日结账单手动/回填。 */
import { ref, onMounted } from 'vue'
import { financeApi } from '@/api'
import { fen2yuan, formatTime } from '@/utils/format'
import { toast } from '@/utils/notify'
import type { ReconcileResult, ReconcileSide, DailyStatement } from '@/types'

// ---- 对账 ----
const checking = ref(false)
const result = ref<ReconcileResult | null>(null)

// ---- 日结 ----
const runForm = ref<{ date: string; days: number }>({ date: '', days: 1 })
const running = ref(false)
const stmts = ref<DailyStatement[]>([])
const stmtTotal = ref(0)
const stmtPage = ref(1)
const stmtPageSize = 15
const stmtLoading = ref(false)

onMounted(() => {
  loadStatements()
})

/** 一键对账:比对订单侧与流水侧的进账/出账总额。 */
async function runReconcile() {
  checking.value = true
  try {
    result.value = await financeApi.reconcile()
  } finally {
    checking.value = false
  }
}

/** 手动日结:date 为空则后端默认昨天;days>1 回填最近 N 天。 */
async function runStatements() {
  const days = Math.min(Math.max(Math.round(runForm.value.days) || 1, 1), 366)
  running.value = true
  try {
    const r = await financeApi.runStatements({
      date: runForm.value.date || undefined,
      days,
    })
    toast(`日结完成,已生成 ${r.done_days} 天账单`)
    stmtPage.value = 1
    await loadStatements()
  } finally {
    running.value = false
  }
}

async function loadStatements() {
  stmtLoading.value = true
  try {
    const data = await financeApi.listStatements({ page: stmtPage.value, page_size: stmtPageSize })
    stmts.value = data.list
    stmtTotal.value = data.total
  } finally {
    stmtLoading.value = false
  }
}

function onStmtPageChange(p: number) {
  stmtPage.value = p
  loadStatements()
}

function sideSummary(s: ReconcileSide): string {
  return `订单侧 ¥${fen2yuan(s.order_total)}(${s.order_count} 笔) / 流水侧 ¥${fen2yuan(s.flow_total)}(${s.flow_count} 笔)`
}
</script>

<template>
  <div>
    <div class="admin-head"><h2>对账管理</h2></div>

    <!-- 一键对账 -->
    <div class="card">
      <div class="card-head">
        <h3>一键对账</h3>
        <el-button type="primary" :loading="checking" @click="runReconcile">立即对账</el-button>
      </div>
      <p class="tip">比对「订单表已支付/已退款总额」与「资金流水进出账总额」,一致即平账。</p>

      <template v-if="result">
        <el-alert
          :type="result.balanced ? 'success' : 'error'"
          :closable="false"
          class="alert"
          show-icon
        >
          <template #title>
            <template v-if="result.balanced">
              ✅ 平账:订单侧与流水侧进账、出账总额均一致(对账时间 {{ formatTime(result.generated_at) }})
            </template>
            <template v-else>⚠️ 不平账,存在差额,请核对下方明细(对账时间 {{ formatTime(result.generated_at) }})</template>
          </template>
        </el-alert>

        <div class="sides">
          <div class="side" :class="{ ok: result.pay_side.diff === 0, bad: result.pay_side.diff !== 0 }">
            <h4>进账(支付)</h4>
            <p>{{ sideSummary(result.pay_side) }}</p>
            <p class="diff">
              差额:
              <span>¥{{ fen2yuan(Math.abs(result.pay_side.diff)) }}</span>
              <em>{{ result.pay_side.diff === 0 ? '一致' : result.pay_side.diff > 0 ? '流水多于订单' : '流水少于订单' }}</em>
            </p>
          </div>
          <div class="side" :class="{ ok: result.refund_side.diff === 0, bad: result.refund_side.diff !== 0 }">
            <h4>出账(退款)</h4>
            <p>{{ sideSummary(result.refund_side) }}</p>
            <p class="diff">
              差额:
              <span>¥{{ fen2yuan(Math.abs(result.refund_side.diff)) }}</span>
              <em>{{ result.refund_side.diff === 0 ? '一致' : result.refund_side.diff > 0 ? '流水多于订单' : '流水少于订单' }}</em>
            </p>
          </div>
        </div>

        <template v-if="!result.balanced && result.mismatches?.length">
          <h4 class="sub-title">差异明细(最多 20 条)</h4>
          <el-table :data="result.mismatches" stripe size="small" class="table">
            <el-table-column label="订单号" width="230">
              <template #default="{ row }"><span class="mono">{{ row.order_no }}</span></template>
            </el-table-column>
            <el-table-column label="订单侧金额" width="130">
              <template #default="{ row }">¥{{ fen2yuan(row.order_amount) }}</template>
            </el-table-column>
            <el-table-column label="流水侧金额" width="130">
              <template #default="{ row }">¥{{ fen2yuan(row.flow_amount) }}</template>
            </el-table-column>
            <el-table-column label="差异原因" min-width="160">
              <template #default="{ row }">{{ row.reason }}</template>
            </el-table-column>
          </el-table>
        </template>
      </template>
    </div>

    <!-- 日结账单 -->
    <div class="card">
      <div class="card-head">
        <h3>日结账单</h3>
        <div class="run-form">
          <el-date-picker
            v-model="runForm.date"
            type="date"
            value-format="YYYY-MM-DD"
            placeholder="结算日期(默认昨天)"
            style="width: 190px"
          />
          <el-input-number v-model="runForm.days" :min="1" :max="366" style="width: 110px" />
          <el-button :loading="running" @click="runStatements">手动日结</el-button>
        </div>
      </div>
      <p class="tip">每天 02:00(北京时间)定时结算前一天;days 设为 N 可回填最近 N 天(幂等,重复执行覆盖更新)。</p>

      <el-table :data="stmts" v-loading="stmtLoading" stripe class="table">
        <el-table-column label="日期" prop="stat_date" width="130" />
        <el-table-column label="进账" width="130">
          <template #default="{ row }">¥{{ fen2yuan(row.pay_amount) }}</template>
        </el-table-column>
        <el-table-column label="出账" width="130">
          <template #default="{ row }">¥{{ fen2yuan(row.refund_amount) }}</template>
        </el-table-column>
        <el-table-column label="净收入" width="130">
          <template #default="{ row }">¥{{ fen2yuan(row.net_amount) }}</template>
        </el-table-column>
        <el-table-column label="进账笔数" prop="pay_count" width="100" />
        <el-table-column label="出账笔数" prop="refund_count" width="100" />
        <el-table-column label="更新时间" min-width="170">
          <template #default="{ row }">{{ formatTime(row.updated_at) }}</template>
        </el-table-column>
      </el-table>

      <div class="pager" v-if="stmtTotal > stmtPageSize">
        <el-pagination
          layout="prev, pager, next"
          :total="stmtTotal"
          :page-size="stmtPageSize"
          :current-page="stmtPage"
          @current-change="onStmtPageChange"
        />
      </div>
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

.card {
  background: #fff;
  border: 1px solid var(--fawn-border);
  border-radius: var(--fawn-radius);
  padding: 16px 18px;
  margin-bottom: 16px;
}

.card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 6px;
}

.card-head h3 {
  margin: 0;
  font-size: 15px;
}

.run-form {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.tip {
  margin: 0 0 12px;
  font-size: 12.5px;
  color: var(--fawn-text-sub);
}

.alert {
  margin-bottom: 12px;
}

.sides {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 12px;
  margin-bottom: 12px;
}

.side {
  border: 1px solid var(--fawn-border);
  border-radius: 10px;
  padding: 12px 14px;
}

.side.ok {
  border-color: var(--el-color-success-light-5);
}

.side.bad {
  border-color: var(--el-color-danger-light-5);
  background: var(--el-color-danger-light-9);
}

.side h4 {
  margin: 0 0 8px;
  font-size: 13.5px;
}

.side p {
  margin: 0 0 4px;
  font-size: 13px;
  color: var(--fawn-text-sub);
}

.side .diff span {
  font-weight: 600;
  color: var(--fawn-text, #333);
}

.side .diff em {
  font-style: normal;
  margin-left: 6px;
  font-size: 12px;
}

.sub-title {
  margin: 4px 0 8px;
  font-size: 13.5px;
}

.table {
  width: 100%;
}

.mono {
  font-family: monospace;
  font-size: 12.5px;
}

.pager {
  display: flex;
  justify-content: center;
  margin-top: 16px;
}
</style>
