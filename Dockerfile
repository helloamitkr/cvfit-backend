# The Go binary is cross-compiled on the host before docker build runs.
# Build context is cvfit-backend/ — expects ./lambda binary to exist.
# See ecr_push.tf and deploy.sh for the build step.

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    chromium \
    chromium-sandbox \
    ca-certificates \
    fonts-liberation \
    fonts-noto-cjk \
    libatk-bridge2.0-0 \
    libgtk-3-0 \
    libnss3 \
    libxss1 \
    && rm -rf /var/lib/apt/lists/*

COPY lambda /lambda

ENV CHROME_PATH=/usr/bin/chromium

ENTRYPOINT ["/lambda"]
