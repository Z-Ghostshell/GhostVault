# Dashboard SPA only — API is `http://<edge>:8989/api` (browser same-origin when using edge). `make dashboard-dev` is separate (Vite :5177).
# Build from repo root: docker build -f dashboard/Dockerfile .
FROM node:24-alpine AS build
WORKDIR /src/dashboard
COPY dashboard/package.json dashboard/package-lock.json ./
RUN npm ci
COPY dashboard/ ./
ARG VITE_SHOW_RETRIEVE_DEBUG=false
ENV VITE_DASHBOARD_BASE=/dashboard/
ENV VITE_API_BASE_URL=/api
ENV VITE_SHOW_RETRIEVE_DEBUG=$VITE_SHOW_RETRIEVE_DEBUG
RUN npm run build

FROM nginx:1.27-alpine
COPY dashboard/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /src/dashboard/dist /usr/share/nginx/html/dashboard
EXPOSE 80
