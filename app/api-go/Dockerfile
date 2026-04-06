FROM golang:1.24-alpine AS builder

WORKDIR /src

COPY go.mod ./
COPY go.sum* ./

RUN go mod download

COPY main.go ./

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/api-go main.go

FROM scratch

WORKDIR /app

COPY --from=builder /out/api-go /app/api-go

EXPOSE 8000

USER 10001

ENTRYPOINT ["/app/api-go"]