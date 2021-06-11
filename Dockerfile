FROM golang:1.16-buster as build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY create_provenance.go create_provenance.go
RUN go build -o /out/create_provenance

FROM gcr.io/distroless/base
COPY --from=build /out/create_provenance /create_provenance
# Code file to execute when the docker container starts up (`entrypoint.sh`)
ENTRYPOINT ["/create_provenance"]
