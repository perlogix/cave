FROM alpine:latest

ENV BUNKER_MODE="dev"

COPY bunker bunker
COPY config.yaml config.yaml
COPY entrypoint.sh entrypoint.sh

EXPOSE 1999 2000 80 443

ENTRYPOINT [ "./entrypoint.sh" ]