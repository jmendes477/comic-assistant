FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod main.go ./
RUN CGO_ENABLED=0 go build -o /comic-assistant .

FROM alpine:3.22

WORKDIR /app
COPY --from=build /comic-assistant /usr/local/bin/comic-assistant
EXPOSE 8080
CMD ["comic-assistant"]
