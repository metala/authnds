#################
# Build Step
#################

FROM golang:1.11 as build

# Setup work env
RUN mkdir /app /tmp/gocode
ADD . /app/
WORKDIR /app


# Required envs for GO
ENV GOPATH=/tmp/gocode
ENV GOOS=linux
ENV GOARCH=amd64
ENV CGO_ENABLED=0

# Install deps
RUN go get -d -v ./...

# Run go-bindata to embed data for API
RUN go get -u github.com/jteeuwen/go-bindata/... && $GOPATH/bin/go-bindata -pkg=main assets && gofmt -w bindata.go

# Build and copy final result
RUN make linux64 && cp ./bin/glauth64 /app/glauth

#################
# Run Step
#################

FROM scratch

# Copy binary from build container
COPY --from=build /app/glauth /app/glauth

# User privileges nobody:nogroup
USER 65534:65534

# Expose web and LDAP ports
EXPOSE 10389 10636 5555

WORKDIR /app
ENTRYPOINT ["./glauth"]
CMD ["-c", "/app/config.toml"]

