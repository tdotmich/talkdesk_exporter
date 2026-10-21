FROM golang:1.27 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" \
    -o /talkdesk_exporter ./cmd/talkdesk_exporter

# Distroless: no shell and no package manager. The process runs as a non-root user.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /talkdesk_exporter /talkdesk_exporter
COPY talkdesk_exporter.yaml /etc/talkdesk_exporter/talkdesk_exporter.yaml

USER nonroot:nonroot
EXPOSE 10060

ENTRYPOINT ["/talkdesk_exporter"]
CMD ["-config.file=/etc/talkdesk_exporter/talkdesk_exporter.yaml"]
