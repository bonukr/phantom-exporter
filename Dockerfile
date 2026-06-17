# syntax=docker/dockerfile:1

# ---- build stage ----
FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/phantom-exporter ./cmd/exporter

# ---- runtime stage ----
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/phantom-exporter /app/phantom-exporter
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/phantom-exporter"]
