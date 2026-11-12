<script setup lang="ts">
/** 商品详情:尺码选择(按商品+尺码展示库存)、数量、加购、立即购买。 */
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { productApi, cartApi } from '@/api'
import { useCartStore } from '@/stores/cart'
import { useUserStore } from '@/stores/user'
import { fen2yuan, AGE_GROUP_NAME, SEASON_NAME } from '@/utils/format'
import { toast } from '@/utils/notify'
import type { ProductDetail } from '@/types'

const route = useRoute()
const router = useRouter()
const cartStore = useCartStore()
const userStore = useUserStore()

const detail = ref<ProductDetail | null>(null)
const loading = ref(true)
const selectedSize = ref('')
const quantity = ref(1)
const adding = ref(false)

const productId = computed(() => Number(route.params.id))
const selectedSKU = computed(() => detail.value?.skus.find(s => s.size === selectedSize.value) ?? null)
const canBuy = computed(() => !!selectedSKU.value && selectedSKU.value.stock > 0 && !adding.value)

onMounted(load)

async function load() {
  loading.value = true
  try {
    detail.value = await productApi.detail(productId.value)
    // 默认选中第一个有货的尺码
    const first = detail.value.skus.find(s => s.stock > 0)
    selectedSize.value = first?.size ?? detail.value.skus[0]?.size ?? ''
  } finally {
    loading.value = false
  }
}

function requireLogin(): boolean {
  if (!userStore.isLoggedIn) {
    router.push({ path: '/login', query: { redirect: route.fullPath } })
    return false
  }
  return true
}

async function addToCart(buyNow = false) {
  if (!requireLogin()) return
  if (!selectedSKU.value) {
    toast('请先选择尺码', 'warning')
    return
  }
  if (selectedSKU.value.stock <= 0) {
    toast('该尺码暂时无货,看看其他尺码吧', 'warning')
    return
  }
  adding.value = true
  try {
    await cartStore.add(detail.value!.id, selectedSize.value, quantity.value)
    if (buyNow) {
      // 立即购买:勾选刚加的项并直接进入结算页
      await cartApi.update(`${detail.value!.id}:${selectedSize.value}`, { selected: true })
      router.push('/checkout')
    } else {
      toast('已加入购物车')
    }
  } finally {
    adding.value = false
  }
}

function stockHint(stock: number): string {
  if (stock <= 0) return '无货'
  if (stock <= 10) return `仅剩 ${stock} 件`
  return '有货'
}
</script>

<template>
  <div v-loading="loading" class="detail-page">
    <template v-if="detail">
      <div class="detail-grid">
        <!-- 左:主图 -->
        <div class="detail-img card">
          <img :src="detail.image" :alt="detail.name" />
        </div>

        <!-- 右:信息与购买 -->
        <div class="detail-info">
          <h1 class="d-name">{{ detail.name }}</h1>
          <div class="d-price">
            <span class="price"><span class="price-symbol">¥</span>{{ fen2yuan(detail.price) }}</span>
            <span class="d-cat">{{ detail.category_name }}</span>
          </div>

          <div class="d-attrs">
            <div class="attr-row">
              <span class="attr-label">年龄段</span>
              <el-tag type="warning" effect="plain" round>{{ AGE_GROUP_NAME[detail.age_group] }}</el-tag>
            </div>
            <div class="attr-row">
              <span class="attr-label">面料材质</span>
              <span>{{ detail.material }}</span>
            </div>
            <div class="attr-row">
              <span class="attr-label">适用季节</span>
              <span>{{ SEASON_NAME[detail.season] }}</span>
            </div>
          </div>

          <!-- 尺码选择(库存按 商品+尺码 维度展示) -->
          <div class="size-section">
            <div class="attr-label">选择尺码</div>
            <div class="size-list">
              <button
                v-for="sku in detail.skus"
                :key="sku.id"
                class="size-btn"
                :class="{
                  active: selectedSize === sku.size,
                  disabled: sku.stock <= 0,
                }"
                :disabled="sku.stock <= 0"
                @click="selectedSize = sku.size"
              >
                <span class="size-label">{{ sku.size }}</span>
                <span class="size-stock">{{ stockHint(sku.stock) }}</span>
              </button>
            </div>
          </div>

          <!-- 数量 -->
          <div class="qty-row">
            <span class="attr-label">购买数量</span>
            <el-input-number v-model="quantity" :min="1" :max="Math.max(selectedSKU?.stock ?? 1, 1)" />
            <span v-if="selectedSKU" class="qty-stock">当前尺码库存 {{ selectedSKU.stock }} 件</span>
          </div>

          <!-- 操作 -->
          <div class="action-row">
            <el-button size="large" class="cart-btn" :disabled="!canBuy" @click="addToCart(false)">
              <el-icon class="btn-icon"><ShoppingCart /></el-icon>加入购物车
            </el-button>
            <el-button type="primary" size="large" class="buy-btn" :disabled="!canBuy" @click="addToCart(true)">
              立即购买
            </el-button>
          </div>

          <el-alert
            class="pay-hint"
            type="info"
            :closable="false"
            show-icon
            title="下单后 15 分钟内未支付,订单将自动取消并回补库存"
          />
        </div>
      </div>

      <!-- 详情描述 -->
      <div class="desc-card card">
        <h2 class="section-title">商品详情</h2>
        <p class="desc-text">{{ detail.description }}</p>
      </div>
    </template>

    <el-empty v-else-if="!loading" description="商品不存在或已下架">
      <RouterLink to="/"><el-button type="primary">回首页逛逛</el-button></RouterLink>
    </el-empty>
  </div>
</template>

<style scoped>
.detail-grid {
  display: grid;
  grid-template-columns: minmax(280px, 460px) 1fr;
  gap: 22px;
}

.detail-img {
  padding: 0;
  overflow: hidden;
}

.detail-img img {
  width: 100%;
  aspect-ratio: 1;
  object-fit: cover;
  display: block;
}

.d-name {
  margin: 0 0 12px;
  font-size: 22px;
  line-height: 1.4;
}

.d-price {
  display: flex;
  align-items: baseline;
  gap: 12px;
  background: var(--fawn-primary-bg);
  border-radius: 12px;
  padding: 14px 16px;
  margin-bottom: 16px;
}

.d-price .price {
  font-size: 28px;
}

.d-cat {
  color: var(--fawn-text-sub);
  font-size: 13px;
}

.d-attrs {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding-bottom: 14px;
  border-bottom: 1px dashed var(--fawn-border);
}

.attr-row {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 14px;
}

.attr-label {
  color: var(--fawn-text-sub);
  font-size: 13.5px;
  min-width: 64px;
}

.size-section {
  margin: 16px 0 4px;
}

.size-list {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 8px;
}

.size-btn {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  min-width: 72px;
  padding: 8px 12px;
  border: 1.5px solid var(--fawn-border);
  border-radius: 12px;
  background: #fff;
  cursor: pointer;
  transition: all 0.15s;
}

.size-btn:hover:not(.disabled) {
  border-color: var(--fawn-primary-light);
}

.size-btn.active {
  border-color: var(--fawn-primary);
  background: var(--fawn-primary-bg);
}

.size-btn.disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.size-label {
  font-weight: 600;
  font-size: 14.5px;
}

.size-stock {
  font-size: 11.5px;
  color: var(--fawn-text-sub);
}

.size-btn.active .size-stock {
  color: var(--fawn-primary);
}

.qty-row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 16px 0 4px;
}

.qty-stock {
  font-size: 12.5px;
  color: var(--fawn-text-sub);
}

.action-row {
  display: flex;
  gap: 12px;
  margin-top: 18px;
}

.cart-btn,
.buy-btn {
  flex: 1;
  max-width: 220px;
  border-radius: 999px;
  font-weight: 600;
}

.btn-icon {
  margin-right: 6px;
}

.pay-hint {
  margin-top: 16px;
}

.desc-card {
  margin-top: 20px;
}

.desc-text {
  margin: 0;
  line-height: 1.9;
  color: var(--fawn-text);
  font-size: 14.5px;
  white-space: pre-wrap;
}

@media (max-width: 720px) {
  .detail-grid {
    grid-template-columns: 1fr;
  }
  .action-row {
    flex-wrap: wrap;
  }
  .cart-btn,
  .buy-btn {
    max-width: none;
  }
}
</style>
