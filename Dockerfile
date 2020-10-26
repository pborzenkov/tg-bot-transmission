FROM golang:1.15 as build

WORKDIR /go/src
ADD . /go/src

RUN CGO_ENABLED=0 go build \
	-mod readonly \
	-ldflags="-X main.Version=$(git describe --tags --always --dirty)" \
	-o /go/bin/bot \
	./cmd/bot

FROM gcr.io/distroless/static:nonroot

COPY --from=build /go/bin/bot /

USER nonroot

CMD ["/bot"]
