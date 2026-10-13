# ---- stage 1: build ----
FROM golang:1.26-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server

# ---- stage 2: run ----
FROM alpine:3.22
# -D: not set password , -H not create home folder
RUN adduser -D -H app 
COPY --from=build /out/server /server
USER app
EXPOSE 8080
ENTRYPOINT ["/server"]