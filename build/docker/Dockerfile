FROM golang:1.21
WORKDIR /go/src/ruff.io/tio
COPY ./ ./
ARG version
ARG gitCommit
RUN go env -w GOPROXY=https://goproxy.cn,direct
RUN CGO_ENABLED=1 go build -ldflags "-X main.Version=${version} -X main.GitCommit=${gitCommit}" -o tio /go/src/ruff.io/tio/cmd/tio/main.go


FROM ubuntu:22.04

RUN apt update && \
  apt install -y tzdata && \
  rm -rf /var/lib/apt/lists/*
ENV TZ=Asia/Shanghai

WORKDIR /app/

COPY --from=0 /go/src/ruff.io/tio/tio .
COPY --from=0 /go/src/ruff.io/tio/config.yaml .

CMD ./tio
