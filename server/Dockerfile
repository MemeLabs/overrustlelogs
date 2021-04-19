# build stage
FROM golang:1.16-alpine AS build-env
RUN apk --no-cache add build-base git mercurial gcc
ADD . /src
RUN cd /src && go get -v . && go build -o orl-server

# final stage
FROM alpine
WORKDIR /app
COPY --from=build-env /src/orl-server /app/
COPY --from=build-env /src/views/ /app/views/
COPY --from=build-env /src/assets/ /app/assets/
CMD ["./orl-server"]