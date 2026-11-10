# 前端目录布局规范

## 顶层结构

```
src/
├── components/     # 可复用组件，按 feature 建子目录（components/game/）
├── game/           # 与 Vue 隔离的纯逻辑引擎（可选）
├── i18n/           # vue-i18n 初始化 + locales/<语言>.json
├── layouts/        # AuthLayout、MainLayout（平铺）
├── pages/          # 页面，一个 feature 一个目录
├── router/         # 路由配置（唯一入口 index.ts）
├── services/       # axios 核心 + 类型 + 每个域一个文件
├── store/          # 每个域一个 Pinia store
├── styles/         # 全局样式与主题令牌（tokens.scss、main.scss）
└── utils/          # 纯工具函数（format、storage、sound……）
```

2. 不使用 `index.ts` 桶文件（除页面入口）；目录即模块，职责一目了然。
3. 静态资源放 `public/`，按 feature 建子目录（`public/images/banner/`、`public/images/tabbar/`）；`src/assets/` 只放会被构建处理的资源。
4. 页面目录约定：

```
pages/Home/index.vue            # 主页入口
pages/Events/index.vue          # 列表页
pages/Events/Detail.vue         # 子页用命名文件
pages/Auth/Login.vue            # 同 feature 多页用命名文件
pages/Records/GameRecords.vue
```

5. 跨 feature 复用的组件才进 `components/`；只被一个页面使用的私有组件放页面目录内。
6. `utils/` 平铺具名导出；工具变多时按主题建子目录（`utils/format.ts`、`utils/storage.ts`）。
7. 新建全局目录（`composables/`、`types/` 等）须有明确职责说明，避免空目录和"杂物间"。
