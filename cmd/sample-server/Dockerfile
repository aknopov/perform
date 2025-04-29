FROM golang:latest AS build-stage

WORKDIR /app

COPY *.go ./
CD 


RUN go mod init sample-server
RUN go get .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /server.prog

# Deploy the application binary into a lean image
FROM gcr.io/distroless/base-debian11 AS build-release-stage

WORKDIR /

COPY --from=build-stage /server.prog /server.prog

EXPOSE 8080

ENTRYPOINT [ "/server.prog" ]