FROM golang:1.14 as build

WORKDIR /go/src
ADD . /go/src

RUN CGO_ENABLED=0 go build \
	-mod readonly \
	-ldflags="-X main.Version=$(git describe --always --dirty)" \
	-o /go/bin/bot \
	./cmd/bot

FROM gcr.io/distroless/static
COPY --from=build /go/bin/bot /

CMD ["/bot"]
