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
RUN cd /root/src && \
  GIT_COMMIT=$(git rev-list -1 HEAD) && \
  go build -o burnell -ldflags "-X main.gitCommit=$GIT_COMMIT"

######## Start a new stage from scratch #######
FROM alpine

# Default to the root group for openshift compatibility.
RUN adduser -u 10001 -S appuser -G root

# RUN apk update
WORKDIR /home/appuser/bin
RUN mkdir /home/appuser/config/

# Copy the Pre-built binary file and default configuraitons from the previous stage
COPY --from=builder /root/src/burnell /home/appuser/bin
COPY --from=builder /root/config/burnell.yml.template /home/appuser/config/burnell.yml
COPY --from=builder /root/src/unit-test/example_p* /home/appuser/config/

# Copy debug tools
COPY --from=builder /go/bin/gops /home/appuser/bin

# Required for openshift compatibility (the go process writes a config file to the home directory)
RUN chmod g+w /home/appuser
ENV HOME /home/appuser

USER appuser

# Command to run the executable
ENTRYPOINT ["./burnell"]
