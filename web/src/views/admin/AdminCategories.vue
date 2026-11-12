<script setup lang="ts">
/** 分类管理:列表 + 新增/编辑/删除。 */
import { ref, onMounted } from 'vue'
import { adminApi } from '@/api'
import { toast } from '@/utils/notify'
import { ElMessageBox } from 'element-plus'
import type { Category } from '@/types'

const list = ref<Category[]>([])
const loading = ref(false)
const dialogVisible = ref(false)
const saving = ref(false)
const editingId = ref<number | null>(null)
const form = ref({ name: '', sort: 0 })

onMounted(load)

async function load() {
  loading.value = true
  try {
    list.value = await adminApi.listCategories()
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingId.value = null
  form.value = { name: '', sort: list.value.length + 1 }
  dialogVisible.value = true
}

function openEdit(c: Category) {
  editingId.value = c.id
  form.value = { name: c.name, sort: c.sort }
  dialogVisible.value = true
}

async function save() {
  if (!form.value.name.trim()) {
    toast('请填写分类名称', 'warning')
    return
  }
  saving.value = true
  try {
    if (editingId.value) {
      await adminApi.updateCategory(editingId.value, form.value)
      toast('分类已更新')
    } else {
      await adminApi.createCategory(form.value)
      toast('分类已创建')
    }
    dialogVisible.value = false
    await load()
  } finally {
    saving.value = false
  }
}

async function remove(c: Category) {
  await ElMessageBox.confirm(`确定删除分类「${c.name}」?分类下有商品时无法删除`, '删除确认', { type: 'warning' })
  await adminApi.deleteCategory(c.id)
  toast('已删除')
  load()
}
</script>

<template>
  <div>
    <div class="admin-head">
      <h2>分类管理</h2>
      <el-button type="primary" @click="openCreate">+ 新增分类</el-button>
    </div>

    <el-table :data="list" v-loading="loading" stripe class="table">
      <el-table-column label="ID" prop="id" width="80" />
      <el-table-column label="分类名称" prop="name" />
      <el-table-column label="排序值(越小越靠前)" prop="sort" width="180" />
      <el-table-column label="操作" width="180">
        <template #default="{ row }">
          <el-button size="small" @click="openEdit(row)">编辑</el-button>
          <el-button size="small" type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑分类' : '新增分类'" width="420px">
      <el-form label-width="80px">
        <el-form-item label="名称" required>
          <el-input v-model="form.name" maxlength="32" placeholder="如:亲子装" />
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="form.sort" :min="0" :max="9999" />
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

.table {
  background: #fff;
  border-radius: var(--fawn-radius);
}
</style>
