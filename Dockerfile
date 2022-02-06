# Builder
FROM golang:1.17 as build

# Setup work env
RUN mkdir /src
ADD . /src/
WORKDIR /src

# Install deps
RUN go get -d -v ./...

# Build
RUN make linux64

# Runtime
FROM scratch as runtime

# Copy binary from build container
COPY --from=build /src/bin/authnds64 /app/authnds
COPY ./config.toml.example /app/config.toml

# User privileges nobody:nogroup
USER 65534:65534

# Expose LDAP ports
EXPOSE 10389 10636

WORKDIR /app
ENTRYPOINT ["./authnds"]
CMD ["-c", "/app/config.toml"]

