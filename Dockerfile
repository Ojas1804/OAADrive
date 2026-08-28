# --- Build stage ---
FROM golang:1.22-alpine AS build

WORKDIR /src

COPY go.mod ./
COPY go.sum* ./
COPY cmd ./cmd
COPY internal ./internal

# No go.sum is committed yet (dependencies were added without network
# access to the dev machine), so resolve and verify them here instead of
# `go mod download`, which requires an existing go.sum.
RUN go mod tidy

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/server

# --- Runtime stage ---
FROM alpine:3.19

RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/server ./server

EXPOSE 8080
ENTRYPOINT ["./server"]
