FROM ubuntu

RUN apt-get update && apt-get install -y --no-install-recommends \
    golang \
    ca-certificates \
    fuse \
    && rm -rf /var/lib/apt/lists/*

RUN go install github.com/PlakarKorp/plakar/cmd/plakar@main

ENV PATH=$PATH:"/root/go/bin"


COPY ./entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]