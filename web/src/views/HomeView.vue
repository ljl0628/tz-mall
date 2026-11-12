<script setup lang="ts">
/** 首页:搜索(跳分类页)+ 热销榜 + 店长推荐。
 * 商品浏览/筛选统一在「商品分类」页(/categories)完成,首页不再展示全部商品列表。 */
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { productApi } from '@/api'
import type { Product, HotProduct } from '@/types'
import ProductCard from '@/components/ProductCard.vue'

const router = useRouter()

const recommended = ref<Product[]>([])
const hot = ref<HotProduct[]>([])

/** 搜索词:提交后带去分类页筛选。 */
const keyword = ref('')

async function loadHome() {
  const data = await productApi.home()
  recommended.value = data.recommended
  hot.value = data.hot ?? []
}

function search() {
  router.push({ path: '/categories', query: keyword.value.trim() ? { keyword: keyword.value.trim() } : {} })
}

onMounted(loadHome)
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
      <RouterLink to="/categories" class="hero-all">浏览全部商品 ›</RouterLink>
    </div>
  </section>

  <!-- 热销榜:按有效订单销量排序(无销量时展示最新上架) -->
  <template v-if="hot.length">
    <h2 class="section-title">热销榜 🔥</h2>
    <div class="grid">
      <ProductCard v-for="(p, i) in hot" :key="p.id" :product="p" :rank="i + 1" :sold="p.sold" />
    </div>
  </template>

  <!-- 推荐位 -->
  <template v-if="recommended.length">
    <h2 class="section-title">店长推荐</h2>
    <div class="grid">
      <ProductCard v-for="p in recommended" :key="p.id" :product="p" />
    </div>
  </template>
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

.hero-all {
  display: inline-block;
  margin-top: 10px;
  font-size: 13.5px;
  color: var(--fawn-primary);
  text-decoration: none;
  font-weight: 600;
}

.hero-all:hover {
  text-decoration: underline;
}

.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(170px, 1fr));
  gap: 14px;
  margin-bottom: 8px;
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
