FROM ghcr.io/blinklabs-io/go:1.26.3-1 AS build

WORKDIR /code
COPY . .
RUN make build

FROM cgr.dev/chainguard/static AS scls
COPY --from=build /code/scls /bin/
USER nonroot
ENTRYPOINT ["scls"]
