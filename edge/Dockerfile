# Edge nginx only (no dashboard build). Dashboard is the `dashboard` Compose service.
# WITH_DASHBOARD=1: nginx.full.conf.template proxies /dashboard/ → http://dashboard:80
# WITH_DASHBOARD=0: nginx.api-mcp.conf.template (no /dashboard)
# Templates are rendered by the nginx entrypoint with envsubst at container start using
# GV_MCP_PATH_TOKEN from the environment (see docker-compose.yml edge service).
# Build: docker build -f edge/Dockerfile --build-arg WITH_DASHBOARD=1 .
ARG WITH_DASHBOARD=1

FROM nginx:1.27-alpine
ARG WITH_DASHBOARD=1
COPY edge/nginx.full.conf.template /tmp/nginx-full.conf.template
COPY edge/nginx.api-mcp.conf.template /tmp/nginx-api-mcp.conf.template
RUN mkdir -p /etc/nginx/templates \
    && if [ "$WITH_DASHBOARD" = "1" ]; then \
         cp /tmp/nginx-full.conf.template /etc/nginx/templates/default.conf.template; \
       else \
         cp /tmp/nginx-api-mcp.conf.template /etc/nginx/templates/default.conf.template; \
       fi \
    && rm -f /etc/nginx/conf.d/default.conf /tmp/nginx-*.conf.template
EXPOSE 8989
