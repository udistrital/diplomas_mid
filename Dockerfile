FROM golang:1.25-bookworm AS builder

WORKDIR /workspace
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /main .

FROM gcr.io/distroless/base-debian12

WORKDIR /
COPY --from=builder /main /main
COPY conf/app.conf conf/app.conf

ENTRYPOINT ["/main"]
