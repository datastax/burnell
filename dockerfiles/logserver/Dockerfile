# Dockerfile References: https://docs.docker.com/engine/reference/builder/

# Start from the latest golang base image
FROM golang:alpine AS builder

# Add Maintainer Info
LABEL maintainer="kafkaesque.io"

RUN apk --no-cache add build-base git bzr mercurial gcc
WORKDIR /root/
ADD . /root
RUN cd /root/src/logserver && go build -o logcollector

######## Start a new stage from scratch #######
FROM alpine

# RUN apk update
WORKDIR /root/bin

# Copy the Pre-built binary file and default configuraitons from the previous stage
COPY --from=builder /root/src/logserver/logcollector /root/bin

# Command to run the executable
ENTRYPOINT ["./logcollector"]
