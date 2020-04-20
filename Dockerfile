FROM registry.services.mts.ru/docker/alpine/debug:3.11
WORKDIR /bin

COPY pipeliner /bin/pipeliner
COPY cmd/pipeliner/config.yaml /etc/application/config.yaml

EXPOSE 80
CMD ["/bin/pipeliner", "-c", "/etc/application/config.yaml"]
