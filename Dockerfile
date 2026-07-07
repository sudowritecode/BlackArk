FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/blackark-control ./cmd/blackark-control
RUN CGO_ENABLED=0 go build -o /out/blackark-agent ./cmd/blackark-agent

FROM alpine:3.21
RUN adduser -D -u 10001 blackark
COPY --from=build /out/blackark-control /usr/local/bin/
COPY --from=build /out/blackark-agent /usr/local/bin/
USER blackark
EXPOSE 8080
ENTRYPOINT ["blackark-control"]
