<script setup lang="ts">
/** 首页:推荐位 + 分类/年龄段筛选 + 关键词搜索 + 分页商品列表。 */
import { ref, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { productApi } from '@/api'
import type { Category, Product, AgeGroup } from '@/types'
import ProductCard from '@/components/ProductCard.vue'
import { AGE_GROUP_NAME } from '@/utils/format'

const route = useRoute()

const homeCats = ref<Category[]>([])
const recommended = ref<Product[]>([])
const list = ref<(Product & { category_name: string })[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const loading = ref(false)

const keyword = ref('')
const categoryId = ref<number | null>(null)
const ageGroup = ref<AgeGroup | ''>('')

async function loadHome() {
  const data = await productApi.home()
  homeCats.value = data.categories
  recommended.value = data.recommended
}

async function loadList() {
  loading.value = true
  try {
    const data = await productApi.list({
      page: page.value,
      page_size: pageSize,
      category_id: categoryId.value ?? undefined,
      age_group: ageGroup.value || undefined,
      keyword: keyword.value || undefined,
    })
    list.value = data.list
    total.value = data.total
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  loadList()
}

function pickCategory(id: number | null) {
  categoryId.value = id
  page.value = 1
  loadList()
}

function pickAge(key: AgeGroup | '') {
  ageGroup.value = key
  page.value = 1
  loadList()
}

onMounted(() => {
  // 支持从地址栏带入关键词(/?keyword=xxx)
  const kw = route.query.keyword
  if (typeof kw === 'string' && kw) keyword.value = kw
  loadHome()
  loadList()
})

watch(
  () => route.query.keyword,
  kw => {
    if (typeof kw === 'string') {
      keyword.value = kw
      page.value = 1
      loadList()
    }
  },
)

function onPageChange(p: number) {
  page.value = p
  loadList()
}
</script>

<template>
  <!-- 搜索区 -->
  <section class="hero card">
    <div class="hero-text">
      <h1>让每个孩子,都穿得舒服</h1>
      <p>精选 0-12 岁童装 · 纯棉亲肤 · 按年龄段与尺码挑选</p>
    </div>
    <div class="hero-search">
      <el-input
        v-model="keyword"
        placeholder="搜索童装,如:连体衣 / 卫衣 / 羽绒服"
        size="large"
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
  </section>

  <!-- 年龄段快捷筛选 -->
  <div class="age-chips">
    <button
      class="age-chip"
      :class="{ active: ageGroup === '' }"
      @click="pickAge('')"
    >
      全部年龄
    </button>
    <button
      v-for="g in (Object.keys(AGE_GROUP_NAME) as AgeGroup[])"
      :key="g"
      class="age-chip"
      :class="{ active: ageGroup === g }"
      @click="pickAge(g)"
    >
      {{ AGE_GROUP_NAME[g] }}
    </button>
  </div>

  <!-- 推荐位 -->
  <template v-if="recommended.length && page === 1 && !keyword && !categoryId && !ageGroup">
    <h2 class="section-title">店长推荐</h2>
    <div class="grid">
      <ProductCard v-for="p in recommended" :key="p.id" :product="p" />
    </div>
  </template>

  <!-- 分类筛选 -->
  <h2 class="section-title">全部商品</h2>
  <div class="cat-chips">
    <button class="cat-chip" :class="{ active: categoryId === null }" @click="pickCategory(null)">
      全部分类
    </button>
    <button
      v-for="c in homeCats"
      :key="c.id"
      class="cat-chip"
      :class="{ active: categoryId === c.id }"
      @click="pickCategory(c.id)"
    >
      {{ c.name }}
    </button>
  </div>

  <!-- 商品列表 -->
  <div v-loading="loading" class="list-wrap">
    <div v-if="list.length" class="grid">
      <ProductCard v-for="p in list" :key="p.id" :product="p" />
    </div>
    <el-empty v-else-if="!loading" description="没有找到相关商品,换个关键词试试吧" />
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
</template>

<style scoped>
.hero {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  flex-wrap: wrap;
  background: linear-gradient(120deg, #fdf3ec 0%, #fbe7d9 100%);
  border: none;
  padding: 26px 24px;
}

.hero-text h1 {
  margin: 0 0 8px;
  font-size: 24px;
  letter-spacing: 1px;
}

.hero-text p {
  margin: 0;
  color: var(--fawn-text-sub);
  font-size: 14px;
}

.hero-search {
  flex: 1;
  min-width: 260px;
  max-width: 420px;
}

.age-chips {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin: 16px 0 4px;
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

.cat-chips {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 16px;
}

.cat-chip {
  border: none;
  background: #fff;
  color: var(--fawn-text-sub);
  border-radius: 10px;
  padding: 6px 14px;
  font-size: 13.5px;
  cursor: pointer;
  border: 1px solid transparent;
}

.cat-chip:hover {
  color: var(--fawn-primary);
}

.cat-chip.active {
  background: var(--fawn-primary-bg);
  border-color: var(--fawn-primary-light);
  color: var(--fawn-primary);
  font-weight: 600;
}

.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(170px, 1fr));
  gap: 14px;
}

.list-wrap {
  min-height: 200px;
}

.pager {
  display: flex;
  justify-content: center;
  margin-top: 24px;
}

@media (max-width: 640px) {
  .hero {
    padding: 18px 16px;
  }
  .hero-text h1 {
    font-size: 19px;
  }
  .grid {
    grid-template-columns: repeat(2, 1fr);
    gap: 10px;
  }
}
</style>
