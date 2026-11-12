/** 结算页跨页状态:地址页选择收货地址后带回结算页回显。 */
import { defineStore } from 'pinia'
import type { Address } from '@/types'

export const useCheckoutStore = defineStore('checkout', {
  state: () => ({
    selectedAddress: null as Address | null,
  }),
  actions: {
    /** 地址页(选择模式)点「选择」后调用,随即返回上一页 */
    select(a: Address) {
      this.selectedAddress = a
    },
    /** 下单成功后清空,避免残留到下次结算 */
    clear() {
      this.selectedAddress = null
    },
  },
})
