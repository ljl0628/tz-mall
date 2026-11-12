# syntax=docker/dockerfile:1
# ---------- 构建阶段:npm ci + 类型检查 + 产出 dist ----------
FROM node:20-alpine AS builder

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY . .
# vue-tsc 类型检查随构建一起执行,类型错误直接 fail build
RUN npm run build

# ---------- 运行阶段:Nginx 托管静态文件 + 反代 API ----------
FROM nginx:1.27-alpine

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/dist /usr/share/nginx/html

EXPOSE 80
