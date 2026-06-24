import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8082',
        changeOrigin: true,
      },
    },
  },
  build: {
    rollupOptions: {
      input: {
        app: './index.html',
        assistant: './src/embed/assistant-sdk.ts',
      },
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) return undefined

          const antdComponent = id.match(/node_modules\/antd\/es\/([^/]+)/)
          if (antdComponent?.[1]) {
            return `antd-${antdComponent[1]}`
          }
          if (id.includes('/@ant-design/icons/')) return 'antd-icons'
          if (id.includes('/@ant-design/cssinjs/')) return 'antd-cssinjs'
          if (id.includes('/@ant-design/')) return 'antd-shared'

          const rcComponent = id.match(/node_modules\/(?:@rc-component|rc-[^/]+)\/([^/]+)/)
          if (rcComponent) {
            const pkg = id.match(/node_modules\/((?:@rc-component\/[^/]+)|(?:rc-[^/]+))/)?.[1]
            return pkg ? pkg.replace('/', '-') : 'rc-vendor'
          }

          if (id.includes('/react/') || id.includes('/react-dom/') || id.includes('/react-router-dom/')) return 'react-vendor'
          if (id.includes('/axios/') || id.includes('/dayjs/')) return 'utils-vendor'
          return 'vendor'
        },
      },
    },
  },
})
