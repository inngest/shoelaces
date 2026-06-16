FROM golang:1.24-alpine AS build

WORKDIR /shoelaces
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-s -w -extldflags "-static"' -o /tmp/shoelaces ./cmd/shoelaces && \
printf "---\nnetworkMaps:\n" > /tmp/mappings.yaml && \
mkdir -p /tmp/tftp

# Final container has basically nothing in it but the executable
FROM scratch
COPY --from=build /tmp/shoelaces /shoelaces

WORKDIR /data
COPY --from=build /tmp/mappings.yaml mappings.yaml

# TFTP files will be served from /data/tftp; mount or bake them in
COPY --from=build /tmp/tftp /data/tftp
EXPOSE 8081/tcp 69/udp

ENV BIND_ADDR=0.0.0.0:8081
EXPOSE 8081

ENTRYPOINT ["/shoelaces", "-data-dir", "/data"]
CMD []
