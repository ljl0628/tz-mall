/** 购物车状态:条目、勾选、合计,与后端 Redis 购物车保持同步。 */
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { cartApi } from '@/api'
import type { CartItem, CartSummary } from '@/types'

export const useCartStore = defineStore('cart', () => {
  const items = ref<CartItem[]>([])
  const summary = ref<CartSummary>({
    count: 0,
    total_quantity: 0,
    selected_amount: 0,
    selected_quantity: 0,
  })
  const loaded = ref(false)

  const badgeCount = computed(() => summary.value.count)
  const selectedItems = computed(() => items.value.filter(i => i.selected && !i.off_shelf))

  async function fetch() {
    const data = await cartApi.get()
    items.value = data.items
    summary.value = data.summary
    loaded.value = true
  }

  async function add(productId: number, size: string, quantity: number) {
    await cartApi.add({ product_id: productId, size, quantity })
    await fetch()
  }

  async function updateQuantity(item: CartItem, quantity: number) {
    await cartApi.update(`${item.product_id}:${item.size}`, { quantity })
    await fetch()
  }

  async function toggleSelected(item: CartItem, selected: boolean) {
    await cartApi.update(`${item.product_id}:${item.size}`, { selected })
    await fetch()
  }

  async function selectAll(selected: boolean) {
    await cartApi.selectAll(selected)
    await fetch()
  }

  async function remove(item: CartItem) {
    await cartApi.remove(`${item.product_id}:${item.size}`)
    await fetch()
  }

  function reset() {
    items.value = []
    summary.value = { count: 0, total_quantity: 0, selected_amount: 0, selected_quantity: 0 }
    loaded.value = false
  }

  return {
    items, summary, loaded, badgeCount, selectedItems,
    fetch, add, updateQuantity, toggleSelected, selectAll, remove, reset,
  }
})
