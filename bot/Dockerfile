# build stage
FROM golang:alpine AS build-env
RUN apk --no-cache add build-base git bzr mercurial gcc
ADD . /src
RUN cd /src && go mod init orl-bot && go build -o orl-bot

# final stage
FROM alpine
WORKDIR /app
COPY --from=build-env /src/orl-bot /app/
ENTRYPOINT ./orl-bot