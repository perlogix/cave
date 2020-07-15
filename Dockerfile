FROM alpine:latest

ENV CAVE_MODE="dev"

COPY cave cave
COPY config.yaml config.yaml
COPY entrypoint.sh entrypoint.sh

EXPOSE 1999 2000 80 443

ENTRYPOINT [ "./entrypoint.sh" ]