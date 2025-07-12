# Prism plan

This is a **minimal but functional prototype** of the four‑lane task board with colour + shape categories and a Dockerised static host.

---
## 🚀 Quick start (dev)
```bash
# 1. Install deps
npm install
# 2. Start Vite dev server (http://localhost:5173)
npm run dev
```

## 🐳 Build & run with Docker
```bash
# Build production bundle & nginx image
docker build -t time-manager .
# Serve on http://localhost:8080
docker run --rm -p 8080:80 time-manager
```