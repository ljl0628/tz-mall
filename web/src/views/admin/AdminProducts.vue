<script setup lang="ts">
/** 商品管理:列表(搜索/筛选)、新增/编辑对话框(含 SKU 尺码库存)、上下架、删除、改库存。 */
import { ref, reactive, computed, onMounted } from 'vue'
import { adminApi, productApi } from '@/api'
import { fen2yuan, SEASON_NAME } from '@/utils/format'
import { toast } from '@/utils/notify'
import type { Category, Product, ProductInput } from '@/types'
import type { AgeGroup } from '@/types'
import { ElMessageBox } from 'element-plus'

const ageOptions: { key: AgeGroup; label: string }[] = [
  { key: 'baby', label: '婴幼儿 0-3岁' },
  { key: 'little', label: '小童 3-6岁' },
  { key: 'middle', label: '中童 6-9岁' },
  { key: 'big', label: '大童 9-12岁' },
]

const list = ref<(Product & { category_name: string; skus?: { id: number; size: string; stock: number }[] })[]>([])
const categories = ref<Category[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 10
const loading = ref(false)
const keyword = ref('')
const filterStatus = ref<'' | number>('')

const dialogVisible = ref(false)
const editingId = ref<number | null>(null) // null = 新建
const saving = ref(false)

const emptyForm = (): ProductInput => ({
  category_id: 0,
  name: '',
  age_group: 'little',
  material: '纯棉',
  season: 'spring_summer',
  description: '',
  image: '',
  price: 9900,
  status: 1,
  recommended: 0,
  skus: [
    { size: '100', stock: 50 },
    { size: '110', stock: 50 },
    { size: '120', stock: 50 },
  ],
})

const form = reactive<{ data: ProductInput }>({ data: emptyForm() })

/** 价格编辑器以「元」为单位(两位小数),form.data.price 存库仍为分。
 *  数据库存分是正确设计(避免浮点误差),但不该把内部单位暴露给操作者。 */
const priceYuan = computed<number>({
  get: () => form.data.price / 100,
  set: v => { form.data.price = Math.round(v * 100) },
})
/** 编辑时后端返回的真实 SKU(用于直接改库存)。 */
const editingSkus = ref<{ id: number; size: string; stock: number }[]>([])

onMounted(() => {
  loadCategories()
  load()
})

async function loadCategories() {
  categories.value = await productApi.categories()
}

async function load() {
  loading.value = true
  try {
    const data = await adminApi.listProducts({
      page: page.value,
      page_size: pageSize,
      keyword: keyword.value || undefined,
      status: filterStatus.value === '' ? undefined : filterStatus.value,
    })
    list.value = data.list as typeof list.value
    total.value = data.total
    // 并行补齐每行 SKU(当前页最多 10 个请求,管理端可接受)
    await Promise.all(
      list.value.map(async p => {
        try {
          const d = await productApi.detail(p.id)
          p.skus = d.skus.map(s => ({ id: s.id, size: s.size, stock: s.stock }))
        } catch {
          p.skus = []
        }
      }),
    )
  } finally {
    loading.value = false
  }
}

async function openCreate() {
  editingId.value = null
  form.data = emptyForm()
  editingSkus.value = []
  // 分类默认选中第一个:空值 0 没有对应选项,会被 el-select 原样显示成「0」
  if (!categories.value.length) await loadCategories().catch(() => {})
  form.data.category_id = categories.value[0]?.id ?? 0
  dialogVisible.value = true
}

async function openEdit(p: Product & { skus?: { id: number; size: string; stock: number }[] }) {
  editingId.value = p.id
  // 拉取详情获得 SKU 列表
  const detail = await productApi.detail(p.id)
  editingSkus.value = detail.skus.map(s => ({ id: s.id, size: s.size, stock: s.stock }))
  form.data = {
    category_id: p.category_id,
    name: p.name,
    age_group: p.age_group,
    material: p.material,
    season: p.season,
    description: p.description,
    image: p.image,
    price: p.price,
    status: p.status,
    recommended: p.recommended,
    skus: detail.skus.map(s => ({ size: s.size, stock: s.stock })),
  }
  dialogVisible.value = true
}

function addSkuRow() {
  form.data.skus.push({ size: '', stock: 30 })
}

function removeSkuRow(i: number) {
  form.data.skus.splice(i, 1)
}

const uploading = ref(false)

/** 上传前预检:类型与大小(服务端仍按魔数强校验,这里只挡明显错误)。 */
function beforeImageUpload(file: File): boolean {
  const okTypes = ['image/jpeg', 'image/png', 'image/webp']
  if (!okTypes.includes(file.type)) {
    toast('仅支持 jpg/png/webp 图片', 'warning')
    return false
  }
  if (file.size > 5 * 1024 * 1024) {
    toast('图片不能超过 5MB', 'warning')
    return false
  }
  return true
}

/** 自定义上传:后端中转到七牛,成功后 URL 自动回填 image 字段。 */
async function doUploadImage(opt: { file: File }): Promise<void> {
  uploading.value = true
  try {
    const data = await adminApi.uploadImage(opt.file)
    form.data.image = data.url
    toast('图片上传成功', 'success')
  } catch {
    // request 层已统一 toast 错误信息
  } finally {
    uploading.value = false
  }
}

async function save() {
  const f = form.data
  if (!f.name || f.name.length < 2) return toast('请填写商品名称(至少 2 字)', 'warning')
  if (!f.category_id) return toast('请选择分类', 'warning')
  if (!f.price || f.price <= 0) return toast('价格必须大于 0', 'warning')
  const seen = new Set<string>()
  for (const s of f.skus) {
    if (!s.size.trim()) return toast('尺码不能为空', 'warning')
    if (seen.has(s.size.trim())) return toast(`尺码 ${s.size} 重复`, 'warning')
    seen.add(s.size.trim())
  }
  if (f.skus.length === 0) return toast('至少配置一个尺码', 'warning')

  saving.value = true
  try {
    if (editingId.value) {
      await adminApi.updateProduct(editingId.value, f)
      toast('商品已更新')
    } else {
      await adminApi.createProduct(f)
      toast('商品已创建')
    }
    dialogVisible.value = false
    await load()
  } finally {
    saving.value = false
  }
}

async function toggleStatus(p: Product) {
  const input = await buildFullInput(p)
  input.status = input.status === 1 ? 0 : 1
  await adminApi.updateProduct(p.id, input)
  toast(input.status === 1 ? '已上架' : '已下架')
  load()
}

async function toggleRecommended(p: Product) {
  const input = await buildFullInput(p)
  input.recommended = input.recommended === 1 ? 0 : 1
  await adminApi.updateProduct(p.id, input)
  toast(input.recommended === 1 ? '已加入推荐位' : '已移出推荐位')
  load()
}

async function buildFullInput(p: Product): Promise<ProductInput> {
  const detail = await productApi.detail(p.id)
  return {
    category_id: p.category_id,
    name: p.name,
    age_group: p.age_group,
    material: p.material,
    season: p.season,
    description: p.description,
    image: p.image,
    price: p.price,
    status: p.status,
    recommended: p.recommended,
    skus: detail.skus.map(s => ({ size: s.size, stock: s.stock })),
  }
}

async function removeProduct(p: Product) {
  await ElMessageBox.confirm(`确定删除商品「${p.name}」?删除后不可恢复`, '删除确认', { type: 'warning' })
  await adminApi.deleteProduct(p.id)
  toast('已删除')
  load()
}

async function editStock(sku: { id: number; size: string; stock: number }, product: Product) {
  const { value } = await ElMessageBox.prompt(
    `商品「${product.name}」尺码 ${sku.size} 当前库存 ${sku.stock},输入新库存:`,
    '修改库存',
    { inputValue: String(sku.stock), inputPattern: /^\d+$/, inputErrorMessage: '请输入非负整数' },
  )
  await adminApi.updateSKUStock(sku.id, Number(value))
  toast('库存已更新(MySQL 与 Redis 已同步)')
  load()
}

function onPageChange(p: number) {
  page.value = p
  load()
}
</script>

<template>
  <div>
    <div class="admin-head">
      <h2>商品管理</h2>
      <el-button type="primary" @click="openCreate">+ 新增商品</el-button>
    </div>

    <div class="toolbar">
      <el-input v-model="keyword" placeholder="按名称搜索" clearable style="max-width: 220px" @keyup.enter="load" @clear="load" />
      <el-select v-model="filterStatus" style="width: 120px" @change="load">
        <el-option label="全部状态" value="" />
        <el-option label="上架" :value="1" />
        <el-option label="下架" :value="0" />
      </el-select>
      <el-button @click="load">查询</el-button>
    </div>

    <el-table :data="list" v-loading="loading" stripe class="table">
      <el-table-column label="ID" prop="id" width="60" />
      <el-table-column label="商品" min-width="240">
        <template #default="{ row }">
          <div class="p-cell">
            <img :src="row.image" class="p-img" />
            <div class="p-meta">
              <span class="p-name">{{ row.name }}</span>
              <span class="p-sub">
                {{ row.category_name }} · {{ ageOptions.find(g => g.key === row.age_group)?.label }} · {{ SEASON_NAME[row.season] }}
              </span>
            </div>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="价格" width="100">
        <template #default="{ row }">¥{{ fen2yuan(row.price) }}</template>
      </el-table-column>
      <el-table-column label="尺码库存" min-width="180">
        <template #default="{ row }">
          <div class="sku-chips">
            <el-tag
              v-for="sku in row.skus ?? []"
              :key="sku.id"
              size="small"
              :type="sku.stock > 10 ? 'info' : 'warning'"
              class="sku-chip"
              @click="editStock(sku, row)"
            >
              {{ sku.size }}:{{ sku.stock }}
            </el-tag>
            <span v-if="!row.skus?.length" class="no-sku">编辑中查看</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'" size="small">
            {{ row.status === 1 ? '上架' : '下架' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="推荐" width="80">
        <template #default="{ row }">
          <el-switch :model-value="row.recommended === 1" @change="toggleRecommended(row)" />
        </template>
      </el-table-column>
      <el-table-column label="操作" width="240" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="openEdit(row)">编辑</el-button>
          <el-button size="small" @click="toggleStatus(row)">{{ row.status === 1 ? '下架' : '上架' }}</el-button>
          <el-button size="small" type="danger" @click="removeProduct(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager" v-if="total > pageSize">
      <el-pagination layout="prev, pager, next" :total="total" :page-size="pageSize" :current-page="page" @current-change="onPageChange" />
    </div>

    <!-- 新增/编辑对话框 -->
    <el-dialog
      v-model="dialogVisible"
      :title="editingId ? '编辑商品' : '新增商品'"
      width="640px"
      destroy-on-close
    >
      <el-form label-width="90px">
        <el-form-item label="商品名称" required>
          <el-input v-model="form.data.name" maxlength="128" placeholder="如:小童圆领条纹短袖T恤" />
        </el-form-item>
        <div class="form-row">
          <el-form-item label="分类" required>
            <el-select v-model="form.data.category_id" placeholder="选择分类">
              <el-option v-for="c in categories" :key="c.id" :label="c.name" :value="c.id" />
            </el-select>
          </el-form-item>
          <el-form-item label="年龄段" required>
            <el-select v-model="form.data.age_group">
              <el-option v-for="g in ageOptions" :key="g.key" :label="g.label" :value="g.key" />
            </el-select>
          </el-form-item>
        </div>
        <div class="form-row">
          <el-form-item label="价格(元)" required>
            <el-input-number v-model="priceYuan" :min="0.01" :max="999900" :step="1" :precision="2" />
          </el-form-item>
          <el-form-item label="状态">
            <el-switch v-model="form.data.status" :active-value="1" :inactive-value="0" active-text="上架" inactive-text="下架" />
          </el-form-item>
        </div>
        <el-form-item label="店长推荐">
          <el-switch
            v-model="form.data.recommended"
            :active-value="1"
            :inactive-value="0"
            active-text="加入首页推荐位"
            inactive-text="不推荐"
          />
        </el-form-item>
        <div class="form-row">
          <el-form-item label="面料材质" required>
            <el-input v-model="form.data.material" maxlength="64" placeholder="如:100% 精梳棉" />
          </el-form-item>
          <el-form-item label="适用季节" required>
            <el-select v-model="form.data.season">
              <el-option label="春夏" value="spring_summer" />
              <el-option label="秋冬" value="autumn_winter" />
            </el-select>
          </el-form-item>
        </div>
        <el-form-item label="商品图">
          <div class="img-field">
            <el-upload
              class="img-upload"
              accept="image/jpeg,image/png,image/webp"
              :show-file-list="false"
              :before-upload="beforeImageUpload"
              :http-request="doUploadImage"
            >
              <div v-loading="uploading" class="img-box" element-loading-text="上传中">
                <img v-if="form.data.image" :src="form.data.image" class="img-preview" alt="商品图预览" />
                <div v-else class="img-empty">+<br />上传图片</div>
              </div>
            </el-upload>
            <div class="img-input">
              <el-input
                v-model="form.data.image"
                size="small"
                placeholder="图片 URL(上传自动回填,也可手动粘贴;留空自动生成占位图)"
                clearable
              />
              <p class="img-tip">jpg/png/webp,单张 ≤ 5MB,上传后 URL 自动填入并可预览</p>
            </div>
          </div>
        </el-form-item>
        <el-form-item label="商品描述">
          <el-input v-model="form.data.description" type="textarea" :rows="3" maxlength="2000" />
        </el-form-item>

        <el-form-item label="尺码库存" required>
          <div class="sku-editor">
            <div v-for="(s, i) in form.data.skus" :key="i" class="sku-row">
              <el-input v-model="s.size" placeholder="尺码,如 110" style="width: 140px" />
              <el-input-number v-model="s.stock" :min="0" :max="1000000" />
              <el-button text type="danger" @click="removeSkuRow(i)">移除</el-button>
            </div>
            <el-button size="small" @click="addSkuRow">+ 添加尺码</el-button>
            <p class="sku-tip">库存按「商品 + 尺码」维度管理;保存后新尺码立即生效,已有尺码保留现有库存。</p>
          </div>
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
.admin-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 14px;
}

.admin-head h2 {
  margin: 0;
}

/* 商品图上传:左预览框 + 右 URL 输入 */
.img-field {
  display: flex;
  gap: 12px;
  align-items: flex-start;
  width: 100%;
}

.img-upload .el-upload {
  border: 1px dashed var(--el-border-color);
  border-radius: 8px;
  cursor: pointer;
  overflow: hidden;
  transition: border-color 0.2s;
}

.img-upload .el-upload:hover {
  border-color: var(--el-color-primary);
}

.img-box {
  width: 120px;
  height: 120px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--el-fill-color-light);
}

.img-preview {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.img-empty {
  text-align: center;
  line-height: 1.5;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.img-input {
  flex: 1;
  padding-top: 4px;
}

.img-tip {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--el-text-color-secondary);
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

.p-cell {
  display: flex;
  align-items: center;
  gap: 10px;
}

.p-img {
  width: 44px;
  height: 44px;
  border-radius: 8px;
  object-fit: cover;
  flex-shrink: 0;
}

.p-meta {
  display: flex;
  flex-direction: column;
  gap: 3px;
  min-width: 0;
}

.p-name {
  font-size: 13.5px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.p-sub {
  font-size: 12px;
  color: var(--fawn-text-sub);
}

.form-row {
  display: flex;
  gap: 12px;
}

.form-row .el-form-item {
  flex: 1;
}

.sku-editor {
  width: 100%;
}

.sku-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
}

.sku-tip {
  margin: 6px 0 0;
  font-size: 12px;
  color: var(--fawn-text-sub);
}

.pager {
  display: flex;
  justify-content: center;
  margin-top: 16px;
}

.sku-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.sku-chip {
  cursor: pointer;
}

.no-sku {
  font-size: 12px;
  color: var(--fawn-text-sub);
}
</style>
