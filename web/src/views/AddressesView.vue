<script setup lang="ts">
/** 我的收货地址:列表(默认置顶)+ 新增/编辑对话框 + 删除 + 设默认。
 * 选择模式(?select=1,自结算页跳入):每条带「选择」按钮,选中即返回结算页回显。 */
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { addressApi } from '@/api'
import { areaOptions, areaTextToCodes } from '@/utils/area'
import { toast } from '@/utils/notify'
import { useCheckoutStore } from '@/stores/checkout'
import type { Address } from '@/types'

const route = useRoute()
const router = useRouter()
const checkoutStore = useCheckoutStore()

/** 选择模式:标题与操作切换,选中后返回上一页(结算页)。 */
const selectMode = computed(() => route.query.select === '1')

const list = ref<Address[]>([])
const loading = ref(false)
const dialogVisible = ref(false)
const saving = ref(false)
const editingId = ref<number | null>(null) // null = 新增

const emptyForm = () => ({
  receiver: '',
  phone: '',
  area: [] as string[],
  detail: '',
  is_default: false,
})
const form = reactive<{ data: ReturnType<typeof emptyForm> }>({ data: emptyForm() })

onMounted(load)

async function load() {
  loading.value = true
  try {
    list.value = await addressApi.list()
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingId.value = null
  form.data = emptyForm()
  dialogVisible.value = true
}

function openEdit(a: Address) {
  editingId.value = a.id
  form.data = {
    receiver: a.receiver,
    phone: a.phone,
    area: areaTextToCodes(a.province, a.city, a.district) ?? [],
    detail: a.detail,
    is_default: a.is_default,
  }
  dialogVisible.value = true
}

async function save() {
  const f = form.data
  if (f.receiver.trim().length < 2) return toast('请填写收货人姓名', 'warning')
  if (!/^1\d{10}$/.test(f.phone)) return toast('请填写 11 位手机号', 'warning')
  if (f.area.length !== 3) return toast('请选择省 / 市 / 区', 'warning')
  if (f.detail.trim().length < 2) return toast('请填写详细地址', 'warning')

  const labels = f.area.map(code => findLabel(code))
  const body = {
    receiver: f.receiver.trim(),
    phone: f.phone,
    province: labels[0],
    city: labels[1],
    district: labels[2],
    detail: f.detail.trim(),
    is_default: f.is_default,
  }
  saving.value = true
  try {
    if (editingId.value) {
      await addressApi.update(editingId.value, body)
      toast('地址已更新')
    } else {
      await addressApi.create(body)
      toast('地址已添加')
    }
    dialogVisible.value = false
    await load()
  } finally {
    saving.value = false
  }
}

/** 行政区划码 → 文本(仅三级,按层遍历开销可忽略)。 */
function findLabel(code: string): string {
  const walk = (opts: typeof areaOptions, depth: number): string => {
    for (const o of opts) {
      if (o.value === code) return o.label
      if (o.children && depth < 3) {
        const hit = walk(o.children, depth + 1)
        if (hit) return hit
      }
    }
    return ''
  }
  return walk(areaOptions, 1)
}

async function remove(a: Address) {
  const { ElMessageBox } = await import('element-plus')
  try {
    await ElMessageBox.confirm(`删除「${a.receiver}」的这条地址?`, '删除地址', { type: 'warning' })
  } catch {
    return
  }
  await addressApi.remove(a.id)
  toast('地址已删除')
  await load()
}

async function setDefault(a: Address) {
  await addressApi.setDefault(a.id)
  toast('已设为默认地址')
  await load()
}

function fullRegion(a: Address): string {
  return `${a.province} ${a.city} ${a.district}`
}

/** 选择模式:记录选中地址并返回结算页。 */
function choose(a: Address) {
  checkoutStore.select(a)
  router.back()
}
</script>

<template>
  <div class="addr-page">
    <div class="head">
      <h1 class="page-title">{{ selectMode ? '选择收货地址' : '我的收货地址' }}</h1>
      <div class="head-ops">
        <el-button v-if="selectMode" @click="router.back()">返回</el-button>
        <el-button type="primary" round @click="openCreate">+ 新增地址</el-button>
      </div>
    </div>

    <div v-loading="loading" class="list">
      <template v-if="list.length">
        <div
          v-for="a in list"
          :key="a.id"
          class="card addr-card"
          :class="{ default: a.is_default, chosen: selectMode && checkoutStore.selectedAddress?.id === a.id }"
          @click="selectMode && choose(a)"
        >
          <div class="ac-main">
            <p class="ac-line1">
              <span class="ac-receiver">{{ a.receiver }}</span>
              <span class="ac-phone">{{ a.phone.slice(0, 3) }}****{{ a.phone.slice(7) }}</span>
              <el-tag v-if="a.is_default" size="small" type="primary" effect="light" round>默认</el-tag>
              <el-tag
                v-if="selectMode && checkoutStore.selectedAddress?.id === a.id"
                size="small"
                type="success"
                effect="light"
                round
              >
                当前使用
              </el-tag>
            </p>
            <p class="ac-line2">{{ fullRegion(a) }}</p>
            <p class="ac-line3">{{ a.detail }}</p>
          </div>
          <div class="ac-ops" @click.stop>
            <el-button v-if="selectMode" type="primary" size="small" round @click="choose(a)">选择</el-button>
            <el-button v-if="!a.is_default && !selectMode" text size="small" @click="setDefault(a)">设为默认</el-button>
            <el-button v-if="!selectMode" text size="small" @click="openEdit(a)">编辑</el-button>
            <el-button v-if="!selectMode" text size="small" type="danger" @click="remove(a)">删除</el-button>
          </div>
        </div>
      </template>
      <el-empty v-else-if="!loading" description="还没有收货地址,添加一条方便下单">
        <el-button type="primary" @click="openCreate">+ 新增地址</el-button>
      </el-empty>
    </div>

    <el-dialog
      v-model="dialogVisible"
      :title="editingId ? '编辑地址' : '新增地址'"
      width="520px"
      :close-on-click-modal="false"
    >
      <el-form label-width="82px">
        <el-form-item label="收货人" required>
          <el-input v-model="form.data.receiver" placeholder="收货人姓名" maxlength="32" />
        </el-form-item>
        <el-form-item label="手机号" required>
          <el-input v-model="form.data.phone" placeholder="11 位手机号" maxlength="11" />
        </el-form-item>
        <el-form-item label="所在地区" required>
          <el-cascader
            v-model="form.data.area"
            :options="areaOptions"
            placeholder="选择省 / 市 / 区"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item label="详细地址" required>
          <el-input
            v-model="form.data.detail"
            type="textarea"
            :rows="2"
            placeholder="街道、门牌号、小区/写字楼等"
            maxlength="200"
          />
        </el-form-item>
        <el-form-item label="设为默认">
          <el-switch v-model="form.data.is_default" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.addr-page {
  max-width: 760px;
  margin: 0 auto;
}

.head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 4px 0 14px;
}

.head-ops {
  display: flex;
  gap: 10px;
}

.page-title {
  font-size: 20px;
  margin: 0;
}

.list {
  min-height: 160px;
}

.addr-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 16px 18px;
  margin-bottom: 12px;
}

.addr-card.default {
  border-color: var(--fawn-primary);
}

/* 选择模式:整卡可点,hover 提示;当前使用项高亮 */
.addr-card.chosen {
  cursor: pointer;
}

.addr-card.chosen:hover {
  box-shadow: 0 4px 14px rgba(232, 115, 74, 0.14);
}

@media (max-width: 768px) {
  .addr-card {
    flex-direction: column;
    align-items: stretch;
  }
  .ac-ops {
    justify-content: flex-end;
  }
}

.ac-main {
  flex: 1;
  min-width: 0;
}

.ac-line1 {
  margin: 0 0 6px;
  display: flex;
  align-items: center;
  gap: 10px;
}

.ac-receiver {
  font-weight: 600;
  font-size: 15px;
}

.ac-phone {
  color: var(--fawn-text-sub);
  font-size: 13px;
}

.ac-line2,
.ac-line3 {
  margin: 3px 0;
  font-size: 13px;
  color: var(--fawn-text-sub);
}

.ac-line3 {
  color: var(--fawn-text);
}

.ac-ops {
  display: flex;
  flex-shrink: 0;
}
</style>
