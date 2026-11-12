<script setup lang="ts">
/** 结算页:收货信息表单 + 勾选商品清单 + 金额确认;提交后跳转订单详情。 */
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useCartStore } from '@/stores/cart'
import { orderApi } from '@/api'
import { fen2yuan, AGE_GROUP_SHORT } from '@/utils/format'
import { toast } from '@/utils/notify'

const cartStore = useCartStore()
const router = useRouter()

const form = ref({
  receiver: '',
  phone: '',
  address: '',
  remark: '',
})
const submitting = ref(false)

const items = computed(() => cartStore.selectedItems)
const totalAmount = computed(() => cartStore.summary.selected_amount)

onMounted(() => {
  if (!cartStore.loaded) cartStore.fetch().catch(() => {})
})

async function submit() {
  const f = form.value
  if (f.receiver.trim().length < 2) {
    toast('请填写收货人姓名', 'warning')
    return
  }
  if (!/^1\d{10}$/.test(f.phone)) {
    toast('请填写 11 位手机号', 'warning')
    return
  }
  if (f.address.trim().length < 5) {
    toast('请填写详细地址', 'warning')
    return
  }
  if (items.value.length === 0) {
    toast('请先在购物车勾选商品', 'warning')
    return
  }
  submitting.value = true
  try {
    const { order_no } = await orderApi.place({
      receiver: f.receiver.trim(),
      phone: f.phone,
      address: f.address.trim(),
      remark: f.remark.trim() || undefined,
    })
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
      <!-- 收货信息 -->
      <div class="card">
        <h2 class="section-title">收货信息</h2>
        <el-form label-width="82px" class="addr-form">
          <el-form-item label="收货人" required>
            <el-input v-model="form.receiver" placeholder="请输入收货人姓名" maxlength="32" />
          </el-form-item>
          <el-form-item label="手机号" required>
            <el-input v-model="form.phone" placeholder="11 位手机号" maxlength="11" />
          </el-form-item>
          <el-form-item label="收货地址" required>
            <el-input
              v-model="form.address"
              type="textarea"
              :rows="2"
              placeholder="省 / 市 / 区 / 详细地址"
              maxlength="255"
            />
          </el-form-item>
          <el-form-item label="订单备注">
            <el-input v-model="form.remark" placeholder="选填,如:请发顺丰" maxlength="200" />
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
