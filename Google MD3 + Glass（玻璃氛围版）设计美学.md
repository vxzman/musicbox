Google MD3 + Glass（玻璃氛围版）设计美学

---

## 📋 MD3 Glass — LLM Frontend Design Prompt

> **Use this prompt to instruct an AI coding assistant when generating HTML/CSS/React/Vue components that blend Google Material Design 3 with glassmorphism / backdrop-blur atmosphere. The goal: MD3's semantic clarity + atmospheric depth on large screens.**

---

### 🎯 Design Philosophy（设计哲学）

> "Material You — but make it breathe."

Pure MD3 on large screens can feel empty: flat surfaces, vast whitespace, no visual anchor. This variant **adds atmospheric depth without breaking Material semantics**. Glass is used **only as atmosphere** — never on content-critical surfaces. The result: a soft, luminous, premium feel where MD3's paper layers float inside a colored haze.

**Keywords:** 玻璃氛围 / 光斑 / 通透 / 紫调 / 柔和阴影 / 语义层级 / 不抢内容的模糊

---

### 🎨 Color Palette（配色）

| Token | Value | Usage |
|---|---|---|
| `--md-primary` | `#6750a4` | Primary (MD3 baseline purple) |
| `--md-primary-container` | `#eaddff` | Tonal container bg |
| `--md-on-primary` | `#ffffff` | Text on primary |
| `--md-on-primary-container` | `#21005d` | Text on primary-container |
| `--md-secondary` | `#625b71` | Secondary |
| `--md-tertiary` | `#7d5260` | Tertiary accent |
| `--md-surface` | `rgba(255,255,255,0.60)` | **Glass card surface** |
| `--md-surface-solid` | `#ffffff` | Solid fallback |
| `--md-surface-variant` | `#e7e0ec` | Variant bg |
| `--md-outline` | `#79747e` | Outline |
| `--md-outline-variant` | `#cac4d0` | Subtle outline |
| `--ink` | `#1c1b1f` | Primary text |
| `--ink-2` | `#49454f` | Secondary text |
| `--ink-3` | `#79747e` | Tertiary text |
| **Atmosphere** | | |
| `--bg-base` | `linear-gradient(135deg, #f3e8ff 0%, #e8eaff 50%, #fce8ff 100%)` | Page background gradient |
| `--glow-1` | `radial-gradient(circle, rgba(103,80,164,.25), transparent 70%)` | Purple ambient blob |
| `--glow-2` | `radial-gradient(circle, rgba(80,120,210,.20), transparent 70%)` | Blue ambient blob |
| `--glow-3` | `radial-gradient(circle, rgba(210,100,180,.15), transparent 70%)` | Pink ambient blob |

**Rule:** Content surfaces use MD3 semantic tokens. Atmosphere uses desaturated, low-opacity tinted glows. Never pure neon.

---

### 🌫️ Glass & Blur System（玻璃与模糊体系）

This is the **core differentiator** from pure MD3. Three levels:

#### Level 1 — Ambient Background（氛围层）
```css
body {
  background: var(--bg-base);
  position: relative;
  min-height: 100vh;
}
/* Fixed glowing orbs behind everything */
.ambient-glow {
  position: fixed;
  border-radius: 50%;
  filter: blur(60px);
  z-index: 0;
  pointer-events: none;
}
.glow-purple { width: 600px; height: 600px; background: var(--glow-1); top: -100px; left: -100px; }
.glow-blue   { width: 500px; height: 500px; background: var(--glow-2); top: 30%; right: -80px; }
.glow-pink   { width: 400px; height: 400px; background: var(--glow-3); bottom: -60px; left: 30%; }
```

#### Level 2 — Navigation Bar（导航毛玻璃）
```css
.nav {
  background: rgba(255,255,255,0.65);
  backdrop-filter: blur(20px) saturate(160%);
  -webkit-backdrop-filter: blur(20px) saturate(160%);
  border: 1px solid rgba(255,255,255,0.4);
  border-radius: 999px;           /* Pill shape */
  box-shadow: 0 2px 12px rgba(103,80,164,.08);
}
```

#### Level 3 — Glass Cards（玻璃卡片）
```css
.glass-card {
  background: rgba(255,255,255,0.55);
  backdrop-filter: blur(16px) saturate(150%);
  -webkit-backdrop-filter: blur(16px) saturate(150%);
  border: 1px solid rgba(255,255,255,0.45);
  border-radius: 16px;           /* MD3 standard */
  box-shadow:
    0 1px 3px rgba(0,0,0,.04),
    0 4px 12px rgba(103,80,164,.06),
    0 12px 32px rgba(103,80,164,.05);
  transition: transform .2s ease, box-shadow .2s ease, background .2s ease;
}
.glass-card:hover {
  background: rgba(255,255,255,0.68);
  transform: translateY(-2px);
  box-shadow:
    0 2px 4px rgba(0,0,0,.05),
    0 8px 20px rgba(103,80,164,.10),
    0 20px 48px rgba(103,80,164,.08);
}
```

**Glass rules:**
- `blur(14–20px)` + `saturate(150–160%)` — saturation boost keeps colors vivid through blur
- Opacity: **55–68%** for cards — translucent but readable
- **Never** put glass on body text containers (e.g., pricing detail text should be on solid `#fff` or high-opacity surface)
- **Always** provide solid fallback: `@supports not (backdrop-filter: blur(1px)) { background: #ffffff; }`

---

### 📏 Border & Radius（圆角）

| Element | Radius | Note |
|---|---|---|
| Cards / Containers | **16px** | MD3 standard |
| Buttons | **999px** (pill) | Filled/Tonal/Outlined all pill-shaped |
| Nav bar | **999px** | Floating pill nav |
| Dialogs | **28px** | MD3 extra-large radius |
| Chips / Tags | **8px** or 999px | |
| Images | **12px** | Slight rounding |

**No sharp corners.** MD3 Glass is the **roundest** of all three systems.

---

### 🔘 Buttons（按钮 — 三档语义）

All buttons: `border-radius: 999px`, `font-family: 'Roboto', sans-serif`, `font-size: 14px`, `font-weight: 500`, `padding: 10px 24px`.

#### Filled (Primary)
```css
background: #6750a4;
color: #ffffff;
border: none;
box-shadow: 0 1px 2px rgba(103,80,164,.3);
/* Ripple: white, opacity .18 */
```
#### Filled Tonal (Secondary)
```css
background: #eaddff;
color: #21005d;
border: none;
/* Ripple: #6750a4, opacity .15 */
```
#### Outlined (Tertiary)
```css
background: transparent;
color: #6750a4;
border: 1px solid #cac4d0;
/* Ripple: #6750a4, opacity .12 */
```

**Button state rules:**
- Hover: +4% shadow lift, no transform
- Active: shadow collapses slightly
- **Ripple animation mandatory** (see Ripple section below)

---

### 💧 Ripple Effect（涟漪动效）

```css
.ripple-host { position: relative; overflow: hidden; isolation: isolate; }
.ripple-host::before {
  content:""; position:absolute; inset:0;
  background: var(--ripple-color, #6750a4);
  opacity:0; transition: opacity .15s ease; pointer-events:none; z-index:0;
}
.ripple-host:hover::before { opacity:.06; }
.ripple-host:active::before { opacity:.12; }
.ripple-ink {
  position:absolute; border-radius:50%;
  background: var(--ripple-color, #6750a4);
  opacity:.18; transform: translate(-50%,-50%) scale(0);
  pointer-events:none; z-index:1;
  animation: md-ripple .55s cubic-bezier(.2,0,0,1) forwards;
}
@keyframes md-ripple { to { transform: translate(-50%,-50%) scale(1); opacity:0; } }
.btn-filled { --ripple-color: #ffffff; }
.btn-tonal, .btn-outlined, .glass-card { --ripple-color: #6750a4; }
```

JS attachment: pointerdown → compute cursor offset → create `.ripple-ink` span → remove on animationend + 700ms timeout fallback.

---

### 📦 Cards & Containers

Use `.glass-card` (above) for all content cards: feature tiles, pricing plans, product showcase window.

**Pricing plan special state:**
```css
.plan.recommended {
  background: rgba(234,221,255,0.75);    /* Primary container tint, glass */
  border: 2px solid #6750a4;
  box-shadow: 0 0 0 1px #6750a4, 0 4px 16px rgba(103,80,164,.15);
}
```

---

### 📐 Layout & Grid（布局）

| Rule | Value |
|---|---|
| Max content width | 1080–1200px (wider than Smartisan, to let glass breathe) |
| Spacing scale | 8 / 16 / 24 / 32 / 48 / 64 / 80px |
| Section padding | 64–80px vertical, 32px horizontal |
| Alignment | Left-aligned for body text; **H1 may be centered** on hero |
| Grid | 3-column for features, 3-column for pricing |
| Z-index layers | 0: glows → 10: content → 100: nav → 1000: FAB |

---

### 📝 Typography（字体排印）

```css
font-family: 'Roboto', 'PingFang SC', 'Microsoft YaHei', sans-serif;
```

| Level | Size | Weight | Color |
|---|---|---|---|
| H1 (Hero) | 40–48px | 400 (Regular) | `#1c1b1f` |
| H2 (Section) | 24–28px | 400 | `#1c1b1f` |
| H3 (Card title) | 16px | 500 (Medium) | `#1c1b1f` |
| Body | 14px | 400 | `#49454f` |
| Caption | 12px | 400 | `#79747e` |
| Button | 14px | 500 | varies |

Load Roboto via Google Fonts:
```html
<link href="https://fonts.googleapis.com/css2?family=Roboto:wght@400;500&display=swap" rel="stylesheet">
```

---

### 🧭 Navigation

Floating pill nav (the signature MD3 Glass element):
```html
<nav class="nav">
  <div class="nav-inner">
    <span class="logo">Material OS</span>
    <a class="nav-item active">Overview</a>
    <a class="nav-item">Features</a>
    <a class="nav-item">Pricing</a>
  </div>
</nav>
```
```css
.nav {
  position: sticky; top: 16px; z-index: 100;
  display: flex; justify-content: center;
}
.nav-inner {
  display: flex; align-items: center; gap: 8px;
  background: rgba(255,255,255,0.65);
  backdrop-filter: blur(20px) saturate(160%);
  border: 1px solid rgba(255,255,255,0.4);
  border-radius: 999px;
  padding: 8px 16px;
  box-shadow: 0 2px 12px rgba(103,80,164,.08);
}
.nav-item {
  padding: 8px 16px; border-radius: 999px;
  font-size: 14px; color: #49454f; cursor: pointer;
  transition: background .15s;
}
.nav-item.active {
  background: #eaddff; color: #21005d; font-weight: 500;
}
```

---

### 🔘 FAB (Floating Action Button)

```css
.fab {
  position: fixed; right: 32px; bottom: 32px; z-index: 1000;
  width: 56px; height: 56px;
  border-radius: 16px;            /* MD3: 16px, not full circle */
  background: #6750a4; color: #fff;
  border: none; cursor: pointer;
  box-shadow: 0 4px 12px rgba(103,80,164,.3), 0 12px 28px rgba(103,80,164,.2);
  display: flex; align-items: center; justify-content: center;
  font-size: 24px;
}
.fab:hover { box-shadow: 0 6px 16px rgba(103,80,164,.35), 0 16px 36px rgba(103,80,164,.25); }
```

---

### 🚫 What to AVOID

| Don't | Why |
|---|---|
| Full-page glass cards with <40% opacity | Text becomes unreadable |
| Neon/green/cyan glow colors | Breaks warm MD3 palette |
| `blur()` on body text containers | Content must be crisp |
| Sharp corners (0–4px) | MD3 is round; use 16px+ |
| Heavy multi-layer inset shadows | That's Smartisan, not MD3 |
| Ripple without `overflow:hidden` parent | Ripple leaks outside border-radius |
| Solid dark backgrounds | MD3 Glass is a **light-mode** system |
| Animated spring/bounce easing | MD3 ripple uses `cubic-bezier(.2,0,0,1)` — flat deceleration |

---

### 📋 Quick Checklist for LLM Code Generation

- [ ] Page background = **gradient + 3 fixed radial glow orbs** with `filter: blur(60px)`
- [ ] Nav = **floating pill**, `backdrop-filter: blur(20px) saturate(160%)`, sticky top
- [ ] Cards = **`rgba(255,255,255,0.55)` + `blur(16px)` + 3-layer soft shadow**
- [ ] Border radius: **16px** cards, **999px** buttons, **28px** dialogs
- [ ] Buttons: **3 semantic tiers** (Filled / Tonal / Outlined), all pill-shaped
- [ ] **Ripple effect** on every interactive element (pointerdown → ink span → animationend remove)
- [ ] FAB present (bottom-right, 56×56, `border-radius:16px`)
- [ ] Font: **Roboto** (Google Fonts CDN)
- [ ] Primary color: **`#6750a4`** (MD3 baseline purple)
- [ ] Solid fallback for `backdrop-filter` unsupported browsers
- [ ] Content text always on **high-opacity or solid** surface — never directly on ambient glow

---

### 🧩 Minimal Starter Template

```html
<!DOCTYPE html>
<html lang="zh">
<head>
<meta charset="UTF-8">
<link href="https://fonts.googleapis.com/css2?family=Roboto:wght@400;500&display=swap" rel="stylesheet">
<style>
:root{
  --primary:#6750a4; --primary-container:#eaddff;
  --on-primary:#fff; --on-container:#21005d;
  --ink:#1c1b1f; --ink-2:#49454f; --ink-3:#79747e;
}
* { margin:0; padding:0; box-sizing:border-box; }
body{
  font-family:'Roboto','PingFang SC','Microsoft YaHei',sans-serif;
  background: linear-gradient(135deg,#f3e8ff,#e8eaff 50%,#fce8ff);
  min-height:100vh; color:var(--ink);
}
/* Ambient glows */
.glow{position:fixed;border-radius:50%;filter:blur(60px);z-index:0;pointer-events:none}
.g1{width:600px;height:600px;background:radial-gradient(circle,rgba(103,80,164,.25),transparent 70%);top:-100px;left:-100px}
.g2{width:500px;height:500px;background:radial-gradient(circle,rgba(80,120,210,.20),transparent 70%);top:30%;right:-80px}
.g3{width:400px;height:400px;background:radial-gradient(circle,rgba(210,100,180,.15),transparent 70%);bottom:-60px;left:30%}
/* Glass card */
.gc{position:relative;z-index:10;background:rgba(255,255,255,.55);backdrop-filter:blur(16px) saturate(150%);-webkit-backdrop-filter:blur(16px) saturate(150%);border:1px solid rgba(255,255,255,.45);border-radius:16px;padding:28px;box-shadow:0 1px 3px rgba(0,0,0,.04),0 4px 12px rgba(103,80,164,.06),0 12px 32px rgba(103,80,164,.05)}
/* Ripple */
.ripple-host{position:relative;overflow:hidden;isolation:isolate}
.ripple-host::before{content:"";position:absolute;inset:0;background:var(--rc,#6750a4);opacity:0;transition:opacity .15s;pointer-events:none}
.ripple-host:hover::before{opacity:.06}
.ripple-host:active::before{opacity:.12}
.ripple-ink{position:absolute;border-radius:50%;background:var(--rc,#6750a4);opacity:.18;transform:translate(-50%,-50%) scale(0);pointer-events:none;animation:r .55s cubic-bezier(.2,0,0,1) forwards}
@keyframes r{to{transform:translate(-50%,-50%) scale(1);opacity:0}}
</style>
</head>
<body>
<div class="glow g1"></div><div class="glow g2"></div><div class="glow g3"></div>
<div class="gc ripple-host" style="max-width:600px;margin:80px auto;--rc:#6750a4">
  <h1 style="font-size:36px;font-weight:400;margin-bottom:8px">Material You · Glass</h1>
  <p style="color:var(--ink-2);margin-bottom:20px">Atmospheric depth without losing semantic clarity.</p>
  <button class="ripple-host" style="background:var(--primary);color:#fff;border:none;border-radius:999px;padding:10px 24px;font-size:14px;font-weight:500;cursor:pointer;--rc:#fff">Get started</button>
</div>
<script>
document.querySelectorAll('.ripple-host').forEach(h=>{
  h.dataset.rip=1;
  h.addEventListener('pointerdown',e=>{
    const r=h.getBoundingClientRect(),x=e.clientX-r.left,y=e.clientY-r.top;
    const d=Math.hypot(r.width,r.height)*2;
    const ink=document.createElement('span');ink.className='ripple-ink';
    ink.style.cssText=`width:${d}px;height:${d}px;left:${x}px;top:${y}px`;
    h.appendChild(ink);ink.addEventListener('animationend',()=>ink.remove());
    setTimeout(()=>ink.remove(),700);
  });
});
</script>
</body>
</html>
```

---
