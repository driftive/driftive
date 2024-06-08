FROM gcr.io/distroless/static-debian11:nonroot

COPY "./driftive" /usr/local/bin/driftive

ENTRYPOINT ["driftive"]
