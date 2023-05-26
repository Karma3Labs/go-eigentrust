FROM golang:1.20

WORKDIR /app

# COPY go.mod, go.sum and download the dependencies
COPY go.* ./
RUN go mod download

# COPY All things inside the project and build
COPY . .
RUN go build -o /app/build/main cmd/eigentrust/main.go

EXPOSE 80
CMD [ "/app/build/main", "serve" ]
