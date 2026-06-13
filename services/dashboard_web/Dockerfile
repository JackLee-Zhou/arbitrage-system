# services/dashboard_web/Dockerfile
# 多阶段构建：Node builder + nginx 托管静态资源
# 基于 Vision UI Dashboard React (MIT 开源)

# Stage 1: 构建 React 应用
FROM node:18-alpine AS builder
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

# Stage 2: nginx 托管静态资源
FROM nginx:alpine
COPY --from=builder /app/build /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
