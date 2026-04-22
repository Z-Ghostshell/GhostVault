FROM golang:1.25-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /gvsvd ./cmd/gvsvd \
	&& CGO_ENABLED=0 go build -o /gvmcp ./cmd/gvmcp \
	&& CGO_ENABLED=0 go build -o /gvctl ./cmd/gvctl

FROM alpine:3.20
RUN apk add --no-cache ca-certificates \
	&& addgroup -g 65532 ghostvault \
	&& adduser -D -u 65532 -G ghostvault ghostvault
WORKDIR /app
COPY --from=build /gvsvd /usr/local/bin/gvsvd
COPY --from=build /gvmcp /usr/local/bin/gvmcp
COPY --from=build /gvctl /usr/local/bin/gvctl
COPY --from=build /src/configs/gvsvd.yaml /etc/ghostvault/gvsvd.yaml
RUN chown -R ghostvault:ghostvault /app
ENV GV_Tuning_FILE=/etc/ghostvault/gvsvd.yaml
USER ghostvault:ghostvault
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/gvsvd"]
