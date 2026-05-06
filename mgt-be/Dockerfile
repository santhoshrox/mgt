FROM golang:1.25-alpine AS build
WORKDIR /src

COPY mgt-proto/ ./mgt-proto/

COPY mgt-be/go.mod mgt-be/go.sum ./mgt-be/
WORKDIR /src/mgt-be
RUN go mod download

WORKDIR /src
COPY mgt-be/ ./mgt-be/

WORKDIR /src/mgt-be
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/mgt-be ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/mgt-be /mgt-be
EXPOSE 8080 9090
USER nonroot:nonroot
ENTRYPOINT ["/mgt-be"]
