<script setup lang="ts">
/** 用户管理:列表(搜索/角色/状态筛选 + 分页)、封禁解封、角色调整、重置密码。
 *  封禁与角色调整由后端同步写 Redis 覆盖键,对已登录用户的存量 token 即时生效。 */
import { ref, computed, onMounted } from 'vue'
import { ElMessageBox } from 'element-plus'
import { adminApi } from '@/api'
import { formatTime } from '@/utils/format'
import { toast } from '@/utils/notify'
import { useUserStore } from '@/stores/user'
import type { User } from '@/types'

const userStore = useUserStore()
// 自己的行不允许封禁/改角色(后端同样校验);computed 以便 fetchMe 完成后联动
const myID = computed(() => userStore.user?.id)

const keyword = ref('')
const roleFilter = ref('')
const statusFilter = ref('')
const list = ref<User[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 10
const loading = ref(false)
const acting = ref(0) // 正在操作的行 ID(按钮 loading)

onMounted(load)

async function load() {
  loading.value = true
  try {
    const data = await adminApi.listUsers({
      page: page.value,
      page_size: pageSize,
      keyword: keyword.value.trim() || undefined,
      role: roleFilter.value || undefined,
      status: statusFilter.value === '' ? undefined : statusFilter.value,
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

function onPageChange(p: number) {
  page.value = p
  load()
}

/** 封禁/解封:二段确认,防误触 */
async function toggleStatus(u: User) {
  const disabling = u.status === 1
  const action = disabling ? '封禁' : '解封'
  await ElMessageBox.confirm(
    disabling
      ? `确定封禁用户「${u.username}」?封禁后该用户立即退出登录态且无法登录。`
      : `确定解封用户「${u.username}」?解封后该用户可正常登录。`,
    `${action}确认`,
    { type: disabling ? 'warning' : 'info' },
  )
  acting.value = u.id
  try {
    await adminApi.setUserStatus(u.id, disabling ? 0 : 1)
    toast(`已${action}用户 ${u.username}`)
    load()
  } finally {
    acting.value = 0
  }
}

/** 角色调整:升级管理员需更强确认 */
async function toggleRole(u: User) {
  const toAdmin = u.role === 'user'
  await ElMessageBox.confirm(
    toAdmin
      ? `确定将「${u.username}」升级为管理员?其将获得商品/订单/用户等全部管理权限。`
      : `确定将「${u.username}」降为普通用户?其存量登录态的管理权限将立即失效。`,
    '角色调整确认',
    { type: 'warning' },
  )
  acting.value = u.id
  try {
    await adminApi.setUserRole(u.id, toAdmin ? 'admin' : 'user')
    toast(`已将 ${u.username} ${toAdmin ? '升级为管理员' : '降为普通用户'}`)
    load()
  } finally {
    acting.value = 0
  }
}

/** 重置密码:弹窗输入新密码(6-64 位) */
async function resetPassword(u: User) {
  const { value } = await ElMessageBox.prompt(
    `为用户「${u.username}」设置新密码(6-64 位):`,
    '重置密码',
    {
      inputType: 'password',
      inputPattern: /^.{6,64}$/,
      inputErrorMessage: '密码长度需 6-64 位',
    },
  )
  acting.value = u.id
  try {
    await adminApi.resetUserPassword(u.id, value)
    toast(`已重置 ${u.username} 的密码`)
  } finally {
    acting.value = 0
  }
}
</script>

<template>
  <div>
    <div class="admin-head"><h2>用户管理</h2></div>

    <div class="filters">
      <el-input
        v-model="keyword"
        placeholder="按用户名搜索"
        maxlength="32"
        clearable
        class="f-keyword"
        @keyup.enter="search"
        @clear="search"
      />
      <el-select v-model="roleFilter" placeholder="全部角色" clearable class="f-select" @change="search">
        <el-option label="管理员" value="admin" />
        <el-option label="普通用户" value="user" />
      </el-select>
      <el-select v-model="statusFilter" placeholder="全部状态" clearable class="f-select" @change="search">
        <el-option label="正常" value="1" />
        <el-option label="已禁用" value="0" />
      </el-select>
      <el-button type="primary" @click="search">查询</el-button>
    </div>

    <el-table :data="list" v-loading="loading" stripe class="table">
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column label="用户名" min-width="140">
        <template #default="{ row }">
          <span class="mono">{{ row.username }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="nickname" label="昵称" min-width="120" />
      <el-table-column label="角色" width="110">
        <template #default="{ row }">
          <el-tag :type="row.role === 'admin' ? 'danger' : 'info'" size="small">
            {{ row.role === 'admin' ? '管理员' : '普通用户' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'danger'" size="small">
            {{ row.status === 1 ? '正常' : '已禁用' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="注册时间" width="170">
        <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="250" fixed="right">
        <template #default="{ row }">
          <el-button
            size="small"
            :type="row.status === 1 ? 'danger' : 'success'"
            plain
            :disabled="row.id === myID"
            :loading="acting === row.id"
            @click="toggleStatus(row)"
          >
            {{ row.status === 1 ? '封禁' : '解封' }}
          </el-button>
          <el-button
            size="small"
            :type="row.role === 'user' ? 'warning' : 'info'"
            plain
            :disabled="row.id === myID"
            :loading="acting === row.id"
            @click="toggleRole(row)"
          >
            {{ row.role === 'user' ? '设为管理员' : '取消管理员' }}
          </el-button>
          <el-button size="small" :loading="acting === row.id" @click="resetPassword(row)">重置密码</el-button>
        </template>
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

.filters {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}

.f-keyword {
  width: 200px;
}

.f-select {
  width: 130px;
}

.table {
  background: #fff;
  border-radius: var(--fawn-radius);
}

.mono {
  font-family: monospace;
  font-size: 13px;
}

.pager {
  display: flex;
  justify-content: center;
  margin-top: 16px;
}
</style>
