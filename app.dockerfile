FROM golang:1.22 as builder
WORKDIR /build
COPY go.mod . 
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o ./sv ./cmd/server 


FROM scratch
WORKDIR /app
COPY --from=builder ./build/sv ./cmd/server/sv
#COPY ./migrations/.  ./migrations/.
COPY ./migrations/.  ../migrations/.

LABEL autor=ias
ENTRYPOINT ["/app/cmd/server/sv"]