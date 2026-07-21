# GAIA Web Design System & Brand Style Guide

This document defines the visual design system, color tokens, typography, glassmorphism effects, and layout guidelines for building the GAIA project website. Pass this specification along with the hero image (`assets/hero_banner.png`) to any AI coding agent.

---

## 🎨 Visual Aesthetic & Theme

- **Style**: Cyber-Nature & Deep Dark Mode — Modern developer tool aesthetic.
- **Tone**: Premium, high-tech, futuristic yet clean, glowing node networks.
- **Key Visual Elements**: Glassmorphism, neon bioluminescent glows, subtle grid lines, ambient background gradients.

---

## 💎 Color Palette & CSS Tokens

```css
:root {
  /* Backgrounds */
  --bg-dark: #07090e;
  --bg-surface: #0e121d;
  --bg-card: rgba(18, 24, 38, 0.65);
  --bg-card-hover: rgba(26, 35, 56, 0.85);

  /* Neon Accent Colors */
  --neon-cyan: #00f0ff;
  --neon-emerald: #00ff9d;
  --neon-purple: #9d4edd;
  --neon-blue: #3a86ff;

  /* Text Colors */
  --text-primary: #f8fafc;
  --text-secondary: #94a3b8;
  --text-muted: #64748b;

  /* Borders & Glassmorphism */
  --border-subtle: rgba(255, 255, 255, 0.08);
  --border-neon-cyan: rgba(0, 240, 255, 0.25);
  --border-neon-emerald: rgba(0, 255, 157, 0.25);

  /* Shadows & Glow Effects */
  --glow-cyan: 0 0 25px rgba(0, 240, 255, 0.35);
  --glow-emerald: 0 0 25px rgba(0, 255, 157, 0.35);
  --glass-blur: blur(16px);
}
```

---

## ✒️ Typography & Fonts

- **Primary Font (Headings & UI)**: `Outfit`, `Inter`, or `Roboto` (sans-serif)
- **Monospace Font (Code & CLI)**: `JetBrains Mono`, `Fira Code`, or `Cascadia Code`

```html
<!-- Google Fonts CDN -->
<link rel="preconnect" href="https://fonts.googleapis.com">
<link href="https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;600&family=Outfit:wght@400;600;800&display=swap" rel="stylesheet">
```

---

## ✨ CSS Components & UI Patterns

### Glassmorphic Card

```css
.gaia-card {
  background: var(--bg-card);
  backdrop-filter: var(--glass-blur);
  border: 1px solid var(--border-subtle);
  border-radius: 16px;
  padding: 24px;
  transition: all 0.3s ease;
}

.gaia-card:hover {
  background: var(--bg-card-hover);
  border-color: var(--border-neon-cyan);
  box-shadow: var(--glow-cyan);
  transform: translateY(-2px);
}
```

### Hero Title Gradient

```css
.hero-title {
  font-family: 'Outfit', sans-serif;
  font-size: 3.5rem;
  font-weight: 800;
  background: linear-gradient(135deg, #ffffff 30%, var(--neon-cyan) 70%, var(--neon-emerald) 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}
```

### Ambient Background Glows

```css
.bg-glow-orb {
  position: absolute;
  width: 400px;
  height: 400px;
  border-radius: 50%;
  filter: blur(120px);
  pointer-events: none;
  opacity: 0.15;
}
.bg-glow-cyan { background: var(--neon-cyan); top: 10%; left: 20%; }
.bg-glow-emerald { background: var(--neon-emerald); bottom: 15%; right: 20%; }
```

---

## 🤖 Prompt Template to give to any AI Agent

Copy and paste this prompt when instructing an AI agent to build the website:

> "Build a modern landing page for GAIA (Go Autonomous Intelligence Agent). Use the design system defined in `docs/design-system.md` and match the aesthetic of the hero image `assets/hero_banner.png`.  
> The site must have:  
> 1. Deep dark background (`#07090e`) with ambient neon cyan/emerald blurred glows.  
> 2. Glassmorphic cards for features (12 subagents, MoA, Knowledge Graph, GGA Review).  
> 3. Sleek typography using Google Fonts (Outfit & JetBrains Mono).  
> 4. Interactive micro-animations on hover and code snippet previews."
