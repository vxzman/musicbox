import { createApp } from 'vue'
import App from './App.vue'
import './styles.css'

const app = createApp(App)

// MD3 Glass 涟漪：pointerdown → 创建 .ripple-ink 圆 → animationend/超时移除
app.directive('ripple', {
  mounted(el: HTMLElement) {
    el.classList.add('ripple-host')
    el.addEventListener('pointerdown', (e: PointerEvent) => {
      const r = el.getBoundingClientRect()
      const d = Math.hypot(r.width, r.height) * 2
      const ink = document.createElement('span')
      ink.className = 'ripple-ink'
      ink.style.cssText = `width:${d}px;height:${d}px;left:${e.clientX - r.left}px;top:${e.clientY - r.top}px`
      el.appendChild(ink)
      ink.addEventListener('animationend', () => ink.remove())
      setTimeout(() => ink.remove(), 700)
    })
  },
})

app.mount('#app')
