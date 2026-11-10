import { createPinia } from 'pinia'

// 全局唯一 Pinia 实例：main.ts 装配区与组件树外代码（如 api.ts 守卫逻辑）共用
export const pinia = createPinia()
