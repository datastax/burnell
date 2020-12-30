# Dockerfile References: https://docs.docker.com/engine/reference/builder/

# Start from the latest golang base image
FROM golang:alpine AS builder

# Add Maintainer Info
LABEL maintainer="ming luo"
LABEL stage=build

RUN apk --no-cache add build-base git

# Build Delve
RUN go get github.com/google/gops

WORKDIR /root/
ADD . /root
RUN cd /root/src && go build -o burnell

######## Start a new stage from scratch #######
FROM alpine

# RUN apk update
WORKDIR /root/bin
RUN mkdir /root/config/

# Copy the Pre-built binary file and default configuraitons from the previous stage
COPY --from=builder /root/src/burnell /root/bin
COPY --from=builder /root/config/burnell.yml.template /root/config/burnell.yml
COPY --from=builder /root/src/unit-test/example_p* /root/config/

# Copy debug tools
COPY --from=builder /go/bin/gops /root/bin

# Command to run the executable
ENTRYPOINT ["./burnell"]
