# Dockerfile for go server that will be listening on port specified in ./conf/conf.yml
# syntax=docker/dockerfile:1
FROM golang:alpine3.20
RUN mkdir -p ./conf
COPY ./conf/conf.yml ./conf/conf.yml
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /ecr-webhook-receiver
EXPOSE 8080
CMD ["/ecr-webhook-receiver"]
