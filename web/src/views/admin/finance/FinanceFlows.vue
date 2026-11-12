<script setup lang="ts">
/** 财务中心·资金流水:进账/出账账本查询(日期范围/订单号/流水号/方向)。 */
import { ref, onMounted } from 'vue'
import { financeApi } from '@/api'
import { fen2yuan, formatTime, FLOW_DIRECTION_NAME, FLOW_BIZ_NAME } from '@/utils/format'
import type { PayFlow } from '@/types'

const list = ref<PayFlow[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 15
const loading = ref(false)

// 筛选条件
const dateRange = ref<[string, string] | null>(null)
const orderNo = ref('')
const flowNo = ref('')
const direction = ref<0 | 1 | 2>(0)

onMounted(load)

async function load() {
  loading.value = true
  try {
    const data = await financeApi.listFlows({
      page: page.value,
      page_size: pageSize,
      start_date: dateRange.value?.[0],
      end_date: dateRange.value?.[1],
      order_no: orderNo.value.trim() || undefined,
      flow_no: flowNo.value.trim() || undefined,
      direction: direction.value || undefined,
    })
    list.value = data.list
    total.value = data.total
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  load()
}

function reset() {
  dateRange.value = null
  orderNo.value = ''
  flowNo.value = ''
  direction.value = 0
  search()
}

function onPageChange(p: number) {
  page.value = p
  load()
}
</script>

<template>
  <div>
    <div class="admin-head"><h2>资金流水</h2></div>

    <div class="toolbar">
      <el-date-picker
        v-model="dateRange"
        type="daterange"
        value-format="YYYY-MM-DD"
        range-separator="至"
        start-placeholder="开始日期"
        end-placeholder="结束日期"
        style="width: 260px"
      />
      <el-input v-model="orderNo" placeholder="订单号" clearable style="width: 220px" @keyup.enter="search" />
      <el-input v-model="flowNo" placeholder="流水号" clearable style="width: 220px" @keyup.enter="search" />
      <el-select v-model="direction" style="width: 110px" @change="search">
        <el-option label="全部方向" :value="0" />
        <el-option label="进账" :value="1" />
        <el-option label="出账" :value="2" />
      </el-select>
      <el-button type="primary" @click="search">查询</el-button>
      <el-button @click="reset">重置</el-button>
    </div>

    <el-table :data="list" v-loading="loading" stripe class="table">
      <el-table-column label="流水号" width="230">
        <template #default="{ row }"><span class="mono">{{ row.flow_no }}</span></template>
      </el-table-column>
      <el-table-column label="订单号" width="230">
        <template #default="{ row }"><span class="mono">{{ row.order_no }}</span></template>
      </el-table-column>
      <el-table-column label="方向" width="90">
        <template #default="{ row }">
          <el-tag :type="row.direction === 1 ? 'success' : 'danger'" size="small">
            {{ FLOW_DIRECTION_NAME[row.direction] }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="业务" width="80">
        <template #default="{ row }">{{ FLOW_BIZ_NAME[row.biz_type] ?? row.biz_type }}</template>
      </el-table-column>
      <el-table-column label="金额" width="120">
        <template #default="{ row }">
          <span class="price">¥{{ fen2yuan(row.amount) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="时间" width="170">
        <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="备注" min-width="140">
        <template #default="{ row }">{{ row.remark || '-' }}</template>
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

.toolbar {
  display: flex;
  gap: 10px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}

.table {
  background: #fff;
  border-radius: var(--fawn-radius);
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
