/** 全局轻提示(延迟加载 Element Plus,避免在 request.ts 中引入完整 UI 栈)。 */
import { ElMessage } from 'element-plus'

export function toast(msg: string, type: 'success' | 'error' | 'warning' | 'info' = 'success') {
  ElMessage({ message: msg, type, duration: 2500, grouping: true })
}
