FROM golang:1.25 AS build
WORKDIR /src
COPY . .
RUN make build

FROM gcr.io/distroless/base-debian12:nonroot
WORKDIR /app
COPY --from=build /src/bin/mcu-agent /app/mcu-agent
EXPOSE 8080
ENTRYPOINT ["/app/mcu-agent"]
