FROM golang AS build
WORKDIR /usr/src/app
COPY . .
RUN go get && go build
FROM alpine
COPY --from=build /usr/src/app/secret-copy /app
ENTRYPOINT [ "/lib/ld-musl-x86_64.so.1", "/app" ]
