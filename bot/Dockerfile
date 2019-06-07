FROM golang:1.12.5

WORKDIR /go/src/orl-bot

COPY . .

RUN go get -v ./...
RUN go install -v ./...

CMD ["orl-bot"]