<script setup lang="ts">
/** 商品卡片:图、名称、价格、年龄段/季节标签,点击进入详情。
 * 可选 rank(热销榜名次角标)/ sold(已售件数,优先于分类名展示)。 */
import { RouterLink } from 'vue-router'
import type { Product } from '@/types'
import { fen2yuan, AGE_GROUP_SHORT, SEASON_NAME } from '@/utils/format'

defineProps<{ product: Product; rank?: number; sold?: number }>()
</script>

<template>
  <RouterLink :to="`/product/${product.id}`" class="pcard">
    <div class="pcard-img">
      <img :src="product.image" :alt="product.name" loading="lazy" />
      <span class="pcard-age">{{ AGE_GROUP_SHORT[product.age_group] }}</span>
      <span v-if="rank" class="pcard-rank" :class="{ top: rank <= 3 }">{{ rank }}</span>
    </div>
    <div class="pcard-body">
      <p class="pcard-name">{{ product.name }}</p>
      <div class="pcard-meta">
        <span class="tag">{{ SEASON_NAME[product.season] }}</span>
        <span class="tag tag-plain">{{ product.material }}</span>
      </div>
      <div class="pcard-bottom">
        <span class="price">
          <span class="price-symbol">¥</span>{{ fen2yuan(product.price) }}
        </span>
        <span class="pcard-sold" v-if="sold && sold > 0">已售 {{ sold }}</span>
        <span class="pcard-cat" v-else-if="product.category_name">{{ product.category_name }}</span>
      </div>
    </div>
  </RouterLink>
</template>

<style scoped>
.pcard {
  display: block;
  background: #fff;
  border: 1px solid var(--fawn-border);
  border-radius: var(--fawn-radius);
  overflow: hidden;
  text-decoration: none;
  color: var(--fawn-text);
  transition:
    transform 0.2s,
    box-shadow 0.2s;
}

.pcard:hover {
  transform: translateY(-3px);
  box-shadow: 0 10px 24px rgba(232, 115, 74, 0.14);
}

.pcard-img {
  position: relative;
  aspect-ratio: 1;
  background: var(--fawn-primary-bg);
}

.pcard-img img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.pcard-age {
  position: absolute;
  left: 10px;
  top: 10px;
  background: rgba(255, 255, 255, 0.92);
  color: var(--fawn-primary);
  font-size: 12px;
  font-weight: 600;
  padding: 3px 9px;
  border-radius: 999px;
}

/* 热销榜名次角标:前三名主色,其余深色半透明 */
.pcard-rank {
  position: absolute;
  right: 10px;
  top: 10px;
  min-width: 22px;
  height: 22px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(61, 47, 40, 0.6);
  color: #fff;
  font-size: 12.5px;
  font-weight: 700;
  border-radius: 8px;
}

.pcard-rank.top {
  background: var(--fawn-primary);
}

.pcard-body {
  padding: 12px 13px 14px;
}

.pcard-name {
  margin: 0;
  font-size: 14.5px;
  font-weight: 600;
  line-height: 1.45;
  height: 2.9em;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.pcard-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 8px;
}

.tag {
  font-size: 11.5px;
  color: var(--fawn-primary);
  background: var(--fawn-primary-bg);
  border-radius: 6px;
  padding: 2px 7px;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tag-plain {
  color: var(--fawn-text-sub);
  background: #f5f1ec;
}

.pcard-bottom {
  margin-top: 10px;
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 6px;
}

.pcard-bottom .price {
  font-size: 18px;
}

.pcard-cat,
.pcard-sold {
  font-size: 12px;
  color: var(--fawn-text-sub);
  white-space: nowrap;
}
</style>
