import { ConfigEnv, UserConfigExport } from 'vite'
import path from 'path'
import vue from '@vitejs/plugin-vue'
import { createSvgIconsPlugin } from 'vite-plugin-svg-icons'
import Components from 'unplugin-vue-components/vite'
import { AntDesignVueResolver } from 'unplugin-vue-components/resolvers'
import { ViteEjsPlugin } from 'vite-plugin-ejs'
import dns from 'dns'

const setAlias = (alias: [string, string][]) =>
    alias.map((v) => {
        return { find: v[0], replacement: path.resolve(__dirname, v[1]) }
    })
// https://cn.vitejs.dev/config/server-options.html#server-host
dns.setDefaultResultOrder('verbatim')

export default ({ mode }: ConfigEnv): UserConfigExport => {
    return {
        resolve: {
            alias: setAlias([['/@', 'src']])
        },
        plugins: [
            vue(),
            createSvgIconsPlugin({
                iconDirs: [path.resolve(process.cwd(), 'src/icons')],
                symbolId: 'icon-[dir]-[name]'
            }),
            Components({
                dts: 'src/components.d.ts',
                resolvers: [
                    AntDesignVueResolver({
                        // 关闭按组件 CSS 自动注入：main.ts 已全局引入 antd.css。
                        // 注入发生在编译期，启动扫描看不到 → 运行时发现新依赖 →
                        // vite 全量重打依赖（页面长时间转圈），中途重启还会丢缓存再次全量重打
                        importStyle: false
                    })
                ]
            }),
            ViteEjsPlugin({
                PUBLIC_PATH: mode !== 'development' ? 'PUBLIC-PATH-VARIABLE' : ''
            })
        ],
        experimental: {
            renderBuiltUrl(
                filename: string,
                {
                    hostType
                }: {
          hostId: string;
          hostType: 'js' | 'css' | 'html';
          type: 'asset' | 'public';
        }
            ) {
                if (hostType === 'js') {
                    return {
                        runtime: `window.__assetsPathBuilder(${JSON.stringify(filename)})`
                    }
                }

                return filename
            }
        },
        optimizeDeps: {
            // 显式预构建全部裸模块依赖：避免运行时发现新依赖触发重新预构建
            // （重新预构建期间所有模块请求排队，页面表现为长时间转圈）
            include: [
                'vue',
                'vue-router',
                'pinia',
                'vue-demi',
                'vue-request',
                'axios',
                'nprogress',
                'screenfull',
                'ant-design-vue',
                // AntDesignVueResolver 注入的组件导入来自这个子路径，
                // 不在清单里会在运行时被"发现"并触发全量重打
                'ant-design-vue/es',
                'vue-types'
            ]
        },
        server: {
            host: 'localhost',
            port: 6789,
            base: 'admin',
            proxy: {
                '/api': {
                    changeOrigin: true,
                    target: 'http://localhost:36789'

                }
            }
        },
        css: {
            postcss: {
                plugins: [
                    require('autoprefixer'),
                    require('tailwindcss'),
                    require('postcss-simple-vars'),
                    require('postcss-import')
                ]
            }
        }
    }
}
