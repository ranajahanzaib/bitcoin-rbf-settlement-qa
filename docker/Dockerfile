FROM golang:1.25.3-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /rbfqa ./cmd/rbfqa
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl && rm -rf /var/lib/apt/lists/*
COPY scripts/install-core.sh /usr/local/bin/install-core
RUN install-core /opt/bitcoin
COPY --from=build /rbfqa /usr/local/bin/rbfqa
ENV PATH="/opt/bitcoin/bin:${PATH}"
RUN useradd --uid 10001 --create-home runner && mkdir -p /work/artifacts && chown -R runner:runner /work
USER runner
WORKDIR /work
ENTRYPOINT ["rbfqa"]
CMD ["run", "all", "--timeout", "10m"]
