/** 省市区数据:element-china-area-data 的 regionData 供级联选择,
 * 另构建「文本 → 行政区划码」反查表,供编辑地址时回填级联值。 */
import { regionData } from 'element-china-area-data'

export interface AreaOption {
  value: string
  label: string
  children?: AreaOption[]
}

/** 级联选择器选项(省 → 市 → 区)。 */
export const areaOptions = regionData as unknown as AreaOption[]

let reverseMap: Map<string, string> | null = null

/** label 文本 → value 码,惰性构建一次(约 3000 区县,遍历开销可忽略)。 */
function textToCode(): Map<string, string> {
  if (reverseMap) return reverseMap
  reverseMap = new Map()
  const walk = (opts: AreaOption[]) => {
    for (const o of opts) {
      reverseMap!.set(o.label, o.value)
      if (o.children) walk(o.children)
    }
  }
  walk(areaOptions)
  return reverseMap
}

/** 省市区文本 → 级联 value 路径;任一无法匹配返回 null(调用方回退为手填)。 */
export function areaTextToCodes(province: string, city: string, district: string): string[] | null {
  const m = textToCode()
  const p = m.get(province)
  const c = m.get(city)
  const d = m.get(district)
  if (!p || !c || !d) return null
  return [p, c, d]
}
