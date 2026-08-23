FROM golang:1.24.6

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN go build .

CMD ["./cgroups-stat", "-prom"]
