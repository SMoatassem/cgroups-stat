FROM golang:1.25.0

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN apt-get update && apt-get install -y clang llvm libbpf-dev

RUN go generate ./...

RUN go build .

CMD ["./cgroups-stat", "-prom"]
