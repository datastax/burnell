# Dockerfile References: https://docs.docker.com/engine/reference/builder/

# Start from the latest golang base image
FROM golang:alpine AS builder

# Add Maintainer Info
LABEL maintainer="ming luo"
LABEL stage=build

RUN apk --no-cache add build-base git

# Build Delve
RUN go get github.com/google/gops


# non-root

RUN adduser -S appuser -G root

WORKDIR /home/appuser

#USER appuser

ADD . /home/appuser
RUN cd /home/appuser/src && \
  GIT_COMMIT=$(git rev-list -1 HEAD) && \
  go build -o burnell -ldflags "-X main.gitCommit=$GIT_COMMIT"

RUN chown -R appuser:root /home/appuser

######## Start a new stage from scratch #######
FROM alpine


RUN adduser -S appuser -G root
WORKDIR /home/appuser
RUN chown -R appuser:root /home/appuser


# RUN apk update
WORKDIR /home/appuser/bin
RUN mkdir /home/appuser/config/

# Copy the Pre-built binary file and default configuraitons from the previous stage
COPY --from=builder /home/appuser/src/burnell /home/appuser/bin
COPY --from=builder /home/appuser/config/burnell.yml.template /home/appuser/config/burnell.yml
COPY --from=builder /home/appuser/src/unit-test/example_p* /home/appuser/config/

# Copy debug tools
COPY --from=builder /go/bin/gops /home/appuser/bin

RUN chown -R appuser:root /home/appuser

#RUN chmod -R 777 /home/appuser

ENV GOPS_CONFIG_DIR /home/appuser/config

USER appuser

# Command to run the executable
ENTRYPOINT ["./burnell"]
