# build stage
FROM golang:alpine AS build-env
RUN apk --no-cache add build-base git bzr mercurial gcc
ADD . /src
RUN cd /src && go mod init orl-server && go build -o orl-server

# final stage
FROM alpine
WORKDIR /app
COPY --from=build-env /src/orl-server /app/
COPY --from=build-env /src/views/ /app/views/
COPY --from=build-env /src/assets/ /app/assets/
CMD ["./orl-server"]