<script setup lang="ts">
/** 财务中心·经营看板:核心指标卡 + 近 7/30 天收入趋势图。 */
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import * as echarts from 'echarts/core'
import { LineChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { financeApi } from '@/api'
import { fen2yuan } from '@/utils/format'
import type { FinanceDashboard, TrendPoint } from '@/types'

echarts.use([LineChart, GridComponent, LegendComponent, TooltipComponent, CanvasRenderer])

// 图表配色:两系列(进账/出账),已通过色板校验(白底 ≥3:1、CVD ΔE 24.7)
const SERIES_PAY = '#2a78d6'
const SERIES_REFUND = '#eb6834'
const INK_MUTED = '#898781' // 轴/刻度文字
const GRID_LINE = '#e1e0d9' // 发丝网格线

const dashboard = ref<FinanceDashboard | null>(null)
const points = ref<TrendPoint[]>([])
const days = ref<7 | 30>(7)
const showTable = ref(false)
const loading = ref(false)

const chartEl = ref<HTMLDivElement>()
let chart: echarts.ECharts | null = null

onMounted(load)
onUnmounted(() => {
  chart?.dispose()
  chart = null
})

async function load() {
  loading.value = true
  try {
    const [d, t] = await Promise.all([financeApi.dashboard(), financeApi.trend(days.value)])
    dashboard.value = d
    points.value = t
    await nextTick()
    renderChart()
  } finally {
    loading.value = false
  }
}

async function switchDays(d: 7 | 30) {
  days.value = d
  loading.value = true
  try {
    points.value = await financeApi.trend(d)
    renderChart()
  } finally {
    loading.value = false
  }
}

/** 分 → 轴刻度文案:¥ 全额,过万紧凑。 */
function axisYuan(v: number): string {
  const yuan = v / 100
  if (yuan >= 10000) return `${(yuan / 10000).toFixed(1)}万`
  return `${Math.round(yuan)}`
}

function renderChart() {
  if (!chartEl.value) return
  if (!chart) {
    chart = echarts.init(chartEl.value)
  }
  chart.setOption({
    grid: { left: 48, right: 64, top: 40, bottom: 28 },
    legend: {
      top: 4,
      icon: 'circle',
      itemWidth: 8,
      itemHeight: 8,
      textStyle: { color: '#52514e', fontSize: 12.5 },
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'line', lineStyle: { color: GRID_LINE, width: 1 } },
      valueFormatter: (v: number) => `¥${fen2yuan(v)}`,
    },
    xAxis: {
      type: 'category',
      data: points.value.map(p => p.date.slice(5)), // MM-DD
      boundaryGap: false,
      axisLine: { lineStyle: { color: '#c3c2b7' } },
      axisTick: { show: false },
      axisLabel: { color: INK_MUTED, fontSize: 12 },
    },
    yAxis: {
      type: 'value',
      axisLabel: { color: INK_MUTED, fontSize: 12, formatter: axisYuan },
      splitLine: { lineStyle: { color: GRID_LINE, width: 1 } },
    },
    series: [
      {
        name: '进账',
        type: 'line',
        data: points.value.map(p => p.pay_amount),
        color: SERIES_PAY,
        lineStyle: { width: 2 },
        symbol: 'circle',
        symbolSize: 8,
        itemStyle: { borderColor: '#fff', borderWidth: 2 },
        // 只标注末点,避免每点标数
        endLabel: {
          show: true,
          formatter: () => `¥${fen2yuan(points.value[points.value.length - 1]?.pay_amount ?? 0)}`,
          color: '#52514e',
          fontSize: 12,
          distance: 8,
        },
      },
      {
        name: '出账',
        type: 'line',
        data: points.value.map(p => p.refund_amount),
        color: SERIES_REFUND,
        lineStyle: { width: 2 },
        symbol: 'circle',
        symbolSize: 8,
        itemStyle: { borderColor: '#fff', borderWidth: 2 },
      },
    ],
  })
}
</script>

<template>
  <div v-loading="loading">
    <div class="admin-head">
      <h2>经营看板</h2>
      <span class="sub">历史读日结表,今日为实时流水;净收入 = 进账 − 出账</span>
    </div>

    <!-- 指标卡 -->
    <div class="tiles">
      <div class="tile">
        <span class="tile-label">今日销售额</span>
        <span class="tile-value">¥{{ fen2yuan(dashboard?.today_pay ?? 0) }}</span>
      </div>
      <div class="tile">
        <span class="tile-label">今日退款额</span>
        <span class="tile-value">¥{{ fen2yuan(dashboard?.today_refund ?? 0) }}</span>
      </div>
      <div class="tile">
        <span class="tile-label">本月净收入</span>
        <span class="tile-value">¥{{ fen2yuan(dashboard?.month_net ?? 0) }}</span>
      </div>
      <div class="tile">
        <span class="tile-label">待处理退款单</span>
        <span class="tile-value">{{ dashboard?.pending_refunds ?? 0 }}</span>
      </div>
    </div>

    <!-- 趋势图 -->
    <div class="card chart-card">
      <div class="chart-head">
        <h3>收入趋势</h3>
        <div class="chart-tools">
          <el-checkbox v-model="showTable" size="small">数据表</el-checkbox>
          <el-radio-group :model-value="days" size="small" @update:model-value="switchDays($event as 7 | 30)">
            <el-radio-button :value="7">近 7 天</el-radio-button>
            <el-radio-button :value="30">近 30 天</el-radio-button>
          </el-radio-group>
        </div>
      </div>
      <div v-show="!showTable" ref="chartEl" class="chart" />
      <el-table v-if="showTable" :data="points" stripe size="small" class="table">
        <el-table-column label="日期" prop="date" width="120" />
        <el-table-column label="进账" width="120">
          <template #default="{ row }">¥{{ fen2yuan(row.pay_amount) }}</template>
        </el-table-column>
        <el-table-column label="出账" width="120">
          <template #default="{ row }">¥{{ fen2yuan(row.refund_amount) }}</template>
        </el-table-column>
        <el-table-column label="净收入">
          <template #default="{ row }">¥{{ fen2yuan(row.net_amount) }}</template>
        </el-table-column>
      </el-table>
    </div>
  </div>
</template>

<style scoped>
.admin-head {
  display: flex;
  align-items: baseline;
  gap: 12px;
  margin-bottom: 14px;
}

.admin-head h2 {
  margin: 0;
}

.sub {
  font-size: 12.5px;
  color: var(--fawn-text-sub);
}

.tiles {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(170px, 1fr));
  gap: 12px;
  margin-bottom: 14px;
}

.tile {
  background: #fff;
  border: 1px solid var(--fawn-border);
  border-radius: var(--fawn-radius);
  padding: 16px 18px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.tile-label {
  font-size: 13px;
  color: var(--fawn-text-sub);
}

.tile-value {
  font-size: 26px;
  font-weight: 600;
  line-height: 1;
}

.chart-card {
  padding: 16px 18px;
  background: #fff;
  border: 1px solid var(--fawn-border);
  border-radius: var(--fawn-radius);
}

.chart-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.chart-head h3 {
  margin: 0;
  font-size: 15px;
}

.chart-tools {
  display: flex;
  align-items: center;
  gap: 14px;
}

.chart {
  width: 100%;
  height: 320px;
}

.table {
  width: 100%;
}
</style>
