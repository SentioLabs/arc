# Stage 1: Build the web UI
FROM node:20-alpine AS web
WORKDIR /web
COPY web/package.json web/bun.lock* ./
RUN npm install
COPY web .
RUN npm run build

# Stage 2: Build the Go binary
FROM golang:1.23-alpine AS go-build
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /web/build ./web/build
RUN CGO_ENABLED=0 go build -o /out/arc-paste ./arc-paste

# Stage 3: Runtime container
FROM gcr.io/distroless/static-debian12
COPY --from=go-build /out/arc-paste /arc-paste
EXPOSE 7433
ENTRYPOINT ["/arc-paste"]
