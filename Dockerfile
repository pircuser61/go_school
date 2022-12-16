FROM registry.services.mts.ru/docker/alpine/debug:3.11

ARG SERVICE_NAME='app'
ARG ARTIFACTS_PATH='artifacts'
ARG CONFIG_PATH='./cmd/pipeliner/config.yaml'

ENV BINARY_NAME ${SERVICE_NAME}
ENV BINARY_PATH ${ARTIFACTS_PATH}
ENV CONFIG_PATH ${CONFIG_PATH}

COPY ${CONFIG_PATH} /etc/application/config.yaml
COPY ${BINARY_PATH}/${BINARY_NAME} /bin/${BINARY_NAME}

CMD /bin/${BINARY_NAME} -c /etc/application/config.yaml
