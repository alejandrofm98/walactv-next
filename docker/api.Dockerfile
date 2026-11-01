FROM golang:1.27.3-alpine AS build

WORKDIR /src
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download
COPY apps/api/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/walactv ./cmd/server

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata ffmpeg \
    && addgroup -S -g 10001 walactv \
    && adduser -S -D -H -u 10001 -G walactv walactv
RUN ffmpeg -hide_banner -encoders 2>/dev/null | grep -q 'libx264' \
    && ffmpeg -hide_banner -encoders 2>/dev/null | grep -q ' aac ' \
    && command -v ffprobe >/dev/null \
    && ffprobe -version >/dev/null 2>&1 \
    && ffmpeg -hide_banner -muxers 2>/dev/null | grep -q 'webvtt' \
    && ffmpeg -hide_banner -decoders 2>/dev/null | grep -q ' subrip ' \
    && ffmpeg -hide_banner -decoders 2>/dev/null | grep -q ' ass ' \
    && ffmpeg -hide_banner -decoders 2>/dev/null | grep -q ' mov_text ' \
    && ffmpeg -hide_banner -decoders 2>/dev/null | grep -q ' webvtt '
WORKDIR /app
RUN mkdir -p /var/lib/walactv-vod && chown walactv:walactv /var/lib/walactv-vod && chmod 0700 /var/lib/walactv-vod
COPY --from=build /out/walactv /usr/local/bin/walactv
COPY --from=build --chown=walactv:walactv /src/migrations ./migrations
USER walactv
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/walactv"]
