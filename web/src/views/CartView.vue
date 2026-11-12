<script setup lang="ts">
/** 购物车:勾选/全选、数量增减、删除、合计,去结算。 */
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useCartStore } from '@/stores/cart'
import { fen2yuan, AGE_GROUP_SHORT } from '@/utils/format'
import { toast } from '@/utils/notify'
import type { CartItem } from '@/types'

const cartStore = useCartStore()
const router = useRouter()
const loading = ref(true)

onMounted(async () => {
  try {
    await cartStore.fetch()
    syncAllSelected()
  } finally {
    loading.value = false
  }
})

const allSelected = ref(false)

function syncAllSelected() {
  const items = cartStore.items
  allSelected.value = items.length > 0 && items.every(i => i.selected)
}

async function toggleAll() {
  const next = !allSelected.value
  await cartStore.selectAll(next)
  allSelected.value = next
}

async function toggleOne(index: number) {
  const item = cartStore.items[index]
  await cartStore.toggleSelected(item, !item.selected)
  syncAllSelected()
}

async function changeQty(item: CartItem, qty: number | undefined) {
  if (!qty || qty < 1) return
  if (qty > item.stock) {
    toast('超出当前尺码库存', 'warning')
    return
  }
  await cartStore.updateQuantity(item, qty)
}

async function removeItem(item: CartItem) {
  await cartStore.remove(item)
  toast('已删除')
  syncAllSelected()
}

function goCheckout() {
  if (cartStore.selectedItems.length === 0) {
    toast('请先勾选要结算的商品', 'warning')
    return
  }
  router.push('/checkout')
}
</script>

<template>
  <div v-loading="loading">
    <h1 class="page-title">购物车</h1>

    <template v-if="cartStore.items.length">
      <div class="card cart-card">
        <!-- 全选行 -->
        <div class="cart-head">
          <el-checkbox v-model="allSelected" @change="toggleAll">全选</el-checkbox>
          <span class="head-info">共 {{ cartStore.summary.total_quantity }} 件</span>
        </div>

        <!-- 购物项列表 -->
        <div class="cart-list">
          <div v-for="(item, idx) in cartStore.items" :key="`${item.product_id}:${item.size}`" class="cart-item">
            <el-checkbox
              :model-value="item.selected"
              :disabled="item.off_shelf"
              @change="toggleOne(idx)"
            />
            <img class="ci-img" :src="item.image" :alt="item.name" @click="$router.push(`/product/${item.product_id}`)" />
            <div class="ci-info">
              <p class="ci-name">{{ item.name }}</p>
              <div class="ci-tags">
                <span class="ci-tag">{{ AGE_GROUP_SHORT[item.age_group] }}</span>
                <span class="ci-tag ci-tag-size">尺码 {{ item.size }}</span>
                <span v-if="item.off_shelf" class="ci-off">已下架</span>
                <span v-else-if="item.stock <= 0" class="ci-off">无货</span>
              </div>
            </div>
            <div class="ci-price price">¥{{ fen2yuan(item.price) }}</div>
            <el-input-number
              :model-value="item.quantity"
              :min="1"
              :max="Math.max(item.stock, 1)"
              size="small"
              :disabled="item.off_shelf"
              @change="(v: number | undefined) => changeQty(item, v)"
            />
            <div class="ci-sub">¥{{ fen2yuan(item.price * item.quantity) }}</div>
            <el-button text type="danger" @click="removeItem(item)">删除</el-button>
          </div>
        </div>
      </div>

      <!-- 结算条 -->
      <div class="card settle-bar">
        <div class="settle-info">
          已选 <b>{{ cartStore.summary.selected_quantity }}</b> 件
          <span class="settle-amount">
            合计:<span class="price">¥{{ fen2yuan(cartStore.summary.selected_amount) }}</span>
          </span>
        </div>
        <el-button
          type="primary"
          size="large"
          class="settle-btn"
          :disabled="cartStore.selectedItems.length === 0"
          @click="goCheckout"
        >
          去结算
        </el-button>
      </div>
    </template>

    <el-empty v-else-if="!loading" description="购物车空空的,去挑几件吧">
      <RouterLink to="/"><el-button type="primary">去逛逛</el-button></RouterLink>
    </el-empty>
  </div>
</template>

<style scoped>
.page-title {
  font-size: 20px;
  margin: 4px 0 16px;
}

.cart-card {
  padding: 14px 18px;
}

.cart-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-bottom: 10px;
  border-bottom: 1px solid var(--fawn-border);
}

.head-info {
  color: var(--fawn-text-sub);
  font-size: 13px;
}

.cart-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px 0;
  border-bottom: 1px dashed var(--fawn-border);
}

.cart-item:last-child {
  border-bottom: none;
}

.ci-img {
  width: 76px;
  height: 76px;
  border-radius: 10px;
  object-fit: cover;
  cursor: pointer;
  flex-shrink: 0;
}

.ci-info {
  flex: 1;
  min-width: 0;
}

.ci-name {
  margin: 0 0 8px;
  font-size: 14.5px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ci-tags {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.ci-tag {
  font-size: 11.5px;
  background: #f5f1ec;
  color: var(--fawn-text-sub);
  border-radius: 6px;
  padding: 2px 8px;
}

.ci-tag-size {
  background: var(--fawn-primary-bg);
  color: var(--fawn-primary);
}

.ci-off {
  font-size: 11.5px;
  color: #d05f38;
  background: #fdece6;
  border-radius: 6px;
  padding: 2px 8px;
}

.ci-price {
  width: 84px;
  text-align: center;
  font-size: 15px;
}

.ci-sub {
  width: 90px;
  text-align: right;
  font-weight: 600;
  font-size: 15px;
}

.settle-bar {
  position: sticky;
  bottom: 12px;
  margin-top: 14px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 18px;
  box-shadow: 0 6px 22px rgba(61, 47, 40, 0.08);
}

.settle-info {
  color: var(--fawn-text-sub);
  font-size: 14px;
}

.settle-info b {
  color: var(--fawn-primary);
  font-size: 16px;
}

.settle-amount {
  margin-left: 14px;
}

.settle-amount .price {
  font-size: 22px;
}

.settle-btn {
  border-radius: 999px;
  min-width: 130px;
  font-weight: 600;
}

@media (max-width: 640px) {
  .cart-item {
    flex-wrap: wrap;
  }
  .ci-price,
  .ci-sub {
    width: auto;
  }
  .settle-bar {
    flex-wrap: wrap;
    gap: 8px;
  }
}
</style>
