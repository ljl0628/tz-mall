<script setup lang="ts">
/** 结算页:收货地址(跳转地址页选择后回显)+ 商品清单 + 金额确认。 */
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useCartStore } from '@/stores/cart'
import { useCheckoutStore } from '@/stores/checkout'
import { addressApi, orderApi } from '@/api'
import { fen2yuan, AGE_GROUP_SHORT } from '@/utils/format'
import { toast } from '@/utils/notify'
import type { Address } from '@/types'

const cartStore = useCartStore()
const checkoutStore = useCheckoutStore()
const router = useRouter()

const addresses = ref<Address[]>([])
const remark = ref('')
const submitting = ref(false)

const items = computed(() => cartStore.selectedItems)
const totalAmount = computed(() => cartStore.summary.selected_amount)
/** 当前生效地址:store 选中项(地址页带回)→ 校验仍存在,否则回落默认。 */
const current = computed<Address | null>(() => {
  const sel = checkoutStore.selectedAddress
  if (sel && addresses.value.some(a => a.id === sel.id)) return sel
  return addresses.value.find(a => a.is_default) ?? addresses.value[0] ?? null
})

/** 清理勾选中的失效商品(已下架/无货):自动取消勾选并同步服务端,
 * 避免结算页看不到商品、提交却被后端整体驳回「已下架」。返回清理数量。 */
async function dropInvalidSelected(): Promise<number> {
  const invalid = cartStore.items.filter(i => i.selected && (i.off_shelf || i.stock <= 0))
  for (const it of invalid) {
    await cartStore.toggleSelected(it, false)
  }
  return invalid.length
}

onMounted(async () => {
  // 结算页强制刷新购物车实时状态(价格/库存/上下架)
  await cartStore.fetch().catch(() => {})
  const removed = await dropInvalidSelected()
  if (removed > 0) toast(`已自动移除 ${removed} 件失效商品(下架/无货)`, 'warning')
  try {
    addresses.value = await addressApi.list()
  } catch {
    /* 未登录等场景由路由守卫兜底 */
  }
})

/** 跳转地址页选择(带 select 标记,地址页展示「选择」按钮)。 */
function goSelect() {
  router.push('/addresses?select=1')
}

function maskPhone(phone: string): string {
  return phone.slice(0, 3) + '****' + phone.slice(7)
}

async function submit() {
  // 提交前再清一次失效商品(页面停留期间被下架的竞态)
  const removed = await dropInvalidSelected()
  if (removed > 0) {
    toast(`已自动移除 ${removed} 件失效商品,请确认清单后重新提交`, 'warning')
    return
  }
  if (items.value.length === 0) {
    toast('请先在购物车勾选商品', 'warning')
    return
  }
  if (!current.value) {
    toast('请先选择收货地址', 'warning')
    return
  }
  const a = current.value
  submitting.value = true
  try {
    const { order_no } = await orderApi.place({
      receiver: a.receiver,
      phone: a.phone,
      province: a.province,
      city: a.city,
      district: a.district,
      detail: a.detail,
      remark: remark.value.trim() || undefined,
    })
    checkoutStore.clear()
    toast('下单成功,请在 15 分钟内完成支付')
    cartStore.fetch().catch(() => {})
    router.push(`/orders/${order_no}`)
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="checkout">
    <h1 class="page-title">确认订单</h1>

    <el-empty v-if="cartStore.loaded && items.length === 0" description="没有勾选的商品">
      <RouterLink to="/cart"><el-button type="primary">回购物车</el-button></RouterLink>
    </el-empty>

    <template v-else>
      <!-- 收货信息:已选地址回显 + 跳转地址页更换 -->
      <div class="card">
        <h2 class="section-title">收货信息</h2>

        <div v-if="current" class="chosen" @click="goSelect">
          <div class="chosen-main">
            <p class="chosen-line1">
              <b>{{ current.receiver }}</b>
              <span class="chosen-phone">{{ maskPhone(current.phone) }}</span>
              <el-tag v-if="current.is_default" size="small" effect="light" round>默认</el-tag>
            </p>
            <p class="chosen-line2">{{ current.province }} {{ current.city }} {{ current.district }}</p>
            <p class="chosen-line3">{{ current.detail }}</p>
          </div>
          <span class="chosen-change">更换地址 ›</span>
        </div>

        <div v-else class="chosen-empty" @click="goSelect">
          <el-icon :size="18"><Plus /></el-icon>
          <span>请选择收货地址</span>
        </div>

        <el-form label-width="82px" class="addr-form">
          <el-form-item label="订单备注">
            <el-input v-model="remark" placeholder="选填,如:请发顺丰" maxlength="200" />
          </el-form-item>
        </el-form>
      </div>

      <!-- 商品清单 -->
      <div class="card list-card">
        <h2 class="section-title">商品清单</h2>
        <div v-for="item in items" :key="`${item.product_id}:${item.size}`" class="co-item">
          <img class="co-img" :src="item.image" :alt="item.name" />
          <div class="co-info">
            <p class="co-name">{{ item.name }}</p>
            <span class="co-meta">{{ AGE_GROUP_SHORT[item.age_group] }} · 尺码 {{ item.size }}</span>
          </div>
          <div class="co-right">
            <span class="co-price price">¥{{ fen2yuan(item.price) }}</span>
            <span class="co-qty">× {{ item.quantity }}</span>
          </div>
        </div>

        <div class="co-total">
          应付金额:<span class="price big">¥{{ fen2yuan(totalAmount) }}</span>
        </div>

        <div class="co-actions">
          <el-button size="large" @click="router.push('/cart')">返回购物车</el-button>
          <el-button type="primary" size="large" class="submit" :loading="submitting" @click="submit">
            提交订单
          </el-button>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.checkout {
  max-width: 760px;
  margin: 0 auto;
}

.page-title {
  font-size: 20px;
  margin: 4px 0 16px;
}

.card {
  margin-bottom: 16px;
}

/* 已选地址回显卡片 */
.chosen {
  display: flex;
  align-items: center;
  gap: 12px;
  border: 1.5px solid var(--fawn-primary);
  background: var(--fawn-primary-bg);
  border-radius: 10px;
  padding: 14px 16px;
  cursor: pointer;
  transition: box-shadow 0.15s;
  margin-bottom: 14px;
}

.chosen:hover {
  box-shadow: 0 4px 14px rgba(232, 115, 74, 0.15);
}

.chosen-main {
  flex: 1;
  min-width: 0;
}

.chosen-line1 {
  margin: 0 0 5px;
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 15px;
}

.chosen-phone {
  color: var(--fawn-text-sub);
  font-size: 13px;
}

.chosen-line2,
.chosen-line3 {
  margin: 2px 0;
  font-size: 13px;
  color: var(--fawn-text-sub);
}

.chosen-line3 {
  color: var(--fawn-text);
}

.chosen-change {
  flex-shrink: 0;
  font-size: 13px;
  color: var(--fawn-primary);
  white-space: nowrap;
}

/* 未选地址占位 */
.chosen-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  border: 1.5px dashed var(--fawn-border);
  border-radius: 10px;
  padding: 22px;
  color: var(--fawn-text-sub);
  cursor: pointer;
  margin-bottom: 14px;
  transition: border-color 0.15s, color 0.15s;
}

.chosen-empty:hover {
  border-color: var(--fawn-primary);
  color: var(--fawn-primary);
}

.addr-form {
  max-width: 520px;
}

.co-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 0;
  border-bottom: 1px dashed var(--fawn-border);
}

.co-img {
  width: 64px;
  height: 64px;
  border-radius: 10px;
  object-fit: cover;
}

.co-info {
  flex: 1;
  min-width: 0;
}

.co-name {
  margin: 0 0 6px;
  font-size: 14px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.co-meta {
  font-size: 12.5px;
  color: var(--fawn-text-sub);
}

.co-right {
  display: flex;
  align-items: baseline;
  gap: 10px;
}

.co-price {
  font-size: 15px;
}

.co-qty {
  color: var(--fawn-text-sub);
  font-size: 13px;
}

.co-total {
  text-align: right;
  padding: 16px 4px 4px;
  font-size: 14.5px;
}

.price.big {
  font-size: 24px;
}

.co-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 10px;
}

.submit {
  min-width: 150px;
  border-radius: 999px;
  font-weight: 600;
}
</style>
