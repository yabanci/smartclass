# syntax=docker/dockerfile:1.6

FROM node:22-alpine AS builder
WORKDIR /src
ENV CI=true

COPY package.json ./
RUN npm install --no-audit --no-fund

COPY tsconfig*.json vite.config.ts tailwind.config.js postcss.config.js index.html ./
COPY src ./src
RUN npm run build

FROM nginx:1.27-alpine
COPY --from=builder /src/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
