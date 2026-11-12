<script setup lang="ts">
/** 商品分类页:分类侧栏(移动端横向滑动)+ 年龄段筛选 + 搜索 + 分页商品列表。
 * 筛选状态全部落在 URL query(cat/age/keyword/page),可分享、可前进后退。 */
import { ref, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { productApi } from '@/api'
import type { Category, Product, AgeGroup } from '@/types'
import ProductCard from '@/components/ProductCard.vue'
import { AGE_GROUP_NAME } from '@/utils/format'

const route = useRoute()
const router = useRouter()

const cats = ref<Category[]>([])
const list = ref<(Product & { category_name: string })[]>([])
const total = ref(0)
const loading = ref(false)
const pageSize = 20

/** URL query 为唯一筛选状态源,computed 派生,避免本地/URL 双源不同步。 */
const catId = computed<number | null>(() => {
  const n = Number(route.query.cat)
  return route.query.cat && Number.isFinite(n) && n > 0 ? n : null
})
const age = computed<AgeGroup | ''>(() => {
  const a = route.query.age
  return typeof a === 'string' && a in AGE_GROUP_NAME ? (a as AgeGroup) : ''
})
const page = computed(() => Math.max(1, Number(route.query.page) || 1))

/** 搜索框绑定值,回车/点按钮才落到 URL(keyword query)。 */
const keywordInput = ref(typeof route.query.keyword === 'string' ? route.query.keyword : '')

/** 修改筛选:合并进现有 query,翻页以外均重置回第 1 页。 */
function go(patch: Record<string, string | number | null>) {
  const q: Record<string, string> = {}
  const merged: Record<string, string | number | null> = {
    cat: catId.value,
    age: age.value || null,
    keyword: typeof route.query.keyword === 'string' ? route.query.keyword : null,
    page: page.value,
    ...patch,
  }
  for (const [k, v] of Object.entries(merged)) {
    if (v !== null && v !== undefined && v !== '') q[k] = String(v)
    if (k === 'page' && v === 1) delete q[k] // 第 1 页不落 URL,保持地址干净
  }
  router.replace({ query: q })
}

function pickCategory(id: number | null) {
  go({ cat: id, page: 1 })
}

function pickAge(key: AgeGroup | '') {
  go({ age: key || null, page: 1 })
}

function search() {
  go({ keyword: keywordInput.value.trim() || null, page: 1 })
}

function onPageChange(p: number) {
  go({ page: p })
  // 翻页后回到列表顶部
  window.scrollTo({ top: 0 })
}

async function loadCats() {
  cats.value = await productApi.categories()
}

async function loadList() {
  loading.value = true
  try {
    const kw = typeof route.query.keyword === 'string' ? route.query.keyword : ''
    const data = await productApi.list({
      page: page.value,
      page_size: pageSize,
      category_id: catId.value ?? undefined,
      age_group: age.value || undefined,
      keyword: kw || undefined,
    })
    list.value = data.list
    total.value = data.total
  } finally {
    loading.value = false
  }
}

loadCats()

// query 变化即重新加载(replace 相同 fullPath 不触发,不会重复请求)
watch(
  () => route.query,
  () => {
    if (typeof route.query.keyword === 'string') keywordInput.value = route.query.keyword
    loadList()
  },
  { immediate: true },
)

/** 当前分类名(面包屑/标题提示用)。 */
const currentCatName = computed(
  () => cats.value.find(c => c.id === catId.value)?.name ?? '全部分类',
)
</script>

<template>
  <div class="cat-page">
    <h1 class="page-title">商品分类</h1>

    <div class="cat-layout">
      <!-- 分类侧栏:桌面竖排吸顶,移动端横向滑动条 -->
      <aside class="cat-side card">
        <button
          class="cat-item"
          :class="{ active: catId === null }"
          @click="pickCategory(null)"
        >
          全部分类
        </button>
        <button
          v-for="c in cats"
          :key="c.id"
          class="cat-item"
          :class="{ active: catId === c.id }"
          @click="pickCategory(c.id)"
        >
          {{ c.name }}
        </button>
      </aside>

      <section class="cat-main">
        <!-- 年龄段 + 搜索 -->
        <div class="cat-toolbar">
          <div class="age-chips">
            <button class="age-chip" :class="{ active: age === '' }" @click="pickAge('')">
              全部年龄
            </button>
            <button
              v-for="(name, key) in AGE_GROUP_NAME"
              :key="key"
              class="age-chip"
              :class="{ active: age === key }"
              @click="pickAge(key)"
            >
              {{ name }}
            </button>
          </div>
          <el-input
            v-model="keywordInput"
            class="cat-search"
            placeholder="搜索商品"
            clearable
            @keyup.enter="search"
            @clear="search"
          >
            <template #append>
              <el-button @click="search" aria-label="搜索">
                <el-icon><Search /></el-icon>
              </el-button>
            </template>
          </el-input>
        </div>

        <h2 class="section-title">
          {{ currentCatName }}
          <span class="title-count" v-if="!loading">共 {{ total }} 件</span>
        </h2>

        <div v-loading="loading" class="list-wrap">
          <div v-if="list.length" class="grid">
            <ProductCard v-for="p in list" :key="p.id" :product="p" />
          </div>
          <el-empty v-else-if="!loading" description="该筛选下暂无商品,换个条件试试" />
        </div>

        <div class="pager" v-if="total > pageSize">
          <el-pagination
            layout="prev, pager, next"
            :total="total"
            :page-size="pageSize"
            :current-page="page"
            @current-change="onPageChange"
          />
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.cat-page {
  max-width: 1200px;
  margin: 0 auto;
  width: 100%;
}

.page-title {
  font-size: 20px;
  margin: 4px 0 16px;
}

.cat-layout {
  display: flex;
  gap: 18px;
  align-items: flex-start;
}

/* 侧栏:桌面竖排吸顶 */
.cat-side {
  width: 172px;
  flex-shrink: 0;
  position: sticky;
  top: 76px;
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 8px;
}

.cat-item {
  border: none;
  background: transparent;
  text-align: left;
  border-radius: 10px;
  padding: 10px 14px;
  font-size: 14px;
  color: var(--fawn-text-sub);
  cursor: pointer;
  white-space: nowrap;
  transition: background 0.15s, color 0.15s;
}

.cat-item:hover {
  color: var(--fawn-primary);
  background: var(--fawn-primary-bg);
}

.cat-item.active {
  background: var(--fawn-primary-bg);
  color: var(--fawn-primary);
  font-weight: 600;
}

.cat-main {
  flex: 1;
  min-width: 0;
}

.cat-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  flex-wrap: wrap;
  margin-bottom: 14px;
}

.age-chips {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.age-chip {
  border: 1px solid var(--fawn-border);
  background: #fff;
  color: var(--fawn-text-sub);
  border-radius: 999px;
  padding: 7px 16px;
  font-size: 13.5px;
  cursor: pointer;
  transition: all 0.2s;
}

.age-chip:hover {
  border-color: var(--fawn-primary-light);
  color: var(--fawn-primary);
}

.age-chip.active {
  background: var(--fawn-primary);
  border-color: var(--fawn-primary);
  color: #fff;
  font-weight: 600;
}

.cat-search {
  width: 240px;
  flex-shrink: 0;
}

.section-title {
  display: flex;
  align-items: baseline;
  gap: 10px;
  font-size: 17px;
  margin: 0 0 14px;
}

.title-count {
  font-size: 13px;
  font-weight: 400;
  color: var(--fawn-text-sub);
}

.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(170px, 1fr));
  gap: 14px;
}

.list-wrap {
  min-height: 240px;
}

.pager {
  display: flex;
  justify-content: center;
  margin-top: 24px;
}

@media (max-width: 768px) {
  .cat-layout {
    flex-direction: column;
    gap: 12px;
  }

  /* 移动端:侧栏变横向滑动条,不占整屏 */
  .cat-side {
    position: static;
    width: 100%;
    flex-direction: row;
    overflow-x: auto;
    padding: 8px 10px;
    gap: 6px;
    scrollbar-width: none;
  }

  .cat-side::-webkit-scrollbar {
    display: none;
  }

  .cat-item {
    padding: 7px 14px;
    white-space: nowrap;
    flex-shrink: 0;
  }

  .cat-toolbar {
    flex-direction: column;
    align-items: stretch;
  }

  .cat-search {
    width: 100%;
  }

  .grid {
    grid-template-columns: repeat(2, 1fr);
    gap: 10px;
  }
}
</style>
