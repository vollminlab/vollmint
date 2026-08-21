# Stage 1: build the React SPA
FROM node:20-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: compile the Go binary with the SPA embedded
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /vollmint ./cmd/vollmint

# Stage 3: runtime
FROM gcr.io/distroless/static:nonroot
COPY --from=build /vollmint /vollmint
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/vollmint"]
CMD ["serve"]
