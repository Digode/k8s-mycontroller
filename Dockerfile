FROM golang:1.20 as builder
WORKDIR /app

COPY . .
RUN go build -o mycontroller

FROM gcr.io/distroless/base-debian10
COPY --from=builder /app/mycontroller /
CMD ["/mycontroller"]