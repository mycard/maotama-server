FROM golang as builder

RUN go env -w GO111MODULE=auto \
  && go env -w CGO_ENABLED=0 \
  && go env -w GOPROXY=https://goproxy.cn,https://gocenter.io,https://goproxy.io,direct \
  && go get golang.org/x/net/websocket

WORKDIR /usr/src/app

COPY ./main.go ./
RUN go build -ldflags "-s -w -extldflags '-static'" -o maotama-server main.go

FROM debian:buster-slim

RUN apt update && \
	apt -y install dnsutils && \
	rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

COPY ./entrypoint.sh /
COPY --from=builder /usr/src/app/maotama-server /usr/bin/

WORKDIR /data

ENV TZ Asia/Shanghai

ENTRYPOINT [ "/entrypoint.sh" ]
CMD [ "/usr/bin/maotama-server" ]
