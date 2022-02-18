# build stage
FROM golang:1.17.7-alpine AS build-env
RUN apk --no-cache add build-base git gcc

RUN mkdir -p /build
COPY ./ /build/

WORKDIR /build/cmd/steward/
RUN go version
RUN go build -o steward

# final stage
FROM alpine

RUN apk update && apk add curl && apk add nmap

WORKDIR /app
COPY --from=build-env /build/cmd/steward/steward /app/

ENV RING_BUFFER_SIZE "1000"
ENV CONFIG_FOLDER "./etc"
ENV SOCKET_FOLDER "./tmp"
ENV TCP_LISTENER ""
ENV HTTP_LISTENER "localhost:8091"
ENV DATABASE_FOLDER "./var/lib"
ENV NODE_NAME ""
ENV BROKER_ADDRESS "127.0.0.1:4222"
ENV NATS_CONN_OPT_TIMEOUT "20"
ENV NATS_CONNECT_RETRY_INTERVAL "10"
ENV NATS_RECONNECT_JITTER "100"
ENV NATS_RECONNECT_JITTER_TLS "1"
ENV PROFILING_PORT ""
ENV PROM_HOST_AND_PORT "127.0.0.1:2111"
ENV DEFAULT_MESSAGE_TIMEOUT 10
ENV DEFAULT_MESSAGE_RETRIES 3
ENV DEFAULT_METHOD_TIMEOUT 10
ENV SUBSCRIBERS_DATA_FOLDER "./var"
ENV CENTRAL_NODE_NAME ""
ENV ROOT_CA_PATH ""
ENV NKEY_SEED_FILE ""
ENV EXPOSE_DATA_FOLDER "127.0.0.1:8090"
ENV ERROR_MESSAGE_RETRIES 3
ENV ERROR_MESSAGE_TIMEOUT 10
ENV COMPRESSION ""
ENV SERIALIZATION ""
ENV SET_BLOCK_PROFILE_RATE "0"
ENV ENABLE_SOCKET "1"
ENV ENABLE_TUI "0"
ENV ENABLE_SIGNATURE_CHECK "0"
ENV IS_CENTRAL_AUTH "0"
ENV ENABLE_DEBUG "0"

ENV START_PUB_REQ_HELLO 60

ENV START_SUB_REQ_ERROR_LOG ""
ENV START_SUB_REQ_HELLO ""
ENV START_SUB_REQ_TO_FILE_APPEND ""
ENV START_SUB_REQ_TO_FILE ""
ENV START_SUB_REQ_COPY_FILE_FROM ""
ENV START_SUB_REQ_COPY_FILE_TO ""
ENV START_SUB_REQ_PING ""
ENV START_SUB_REQ_PONG ""
ENV START_SUB_REQ_CLI_COMMAND ""
ENV START_SUB_REQ_TO_CONSOLE ""
ENV START_SUB_REQ_HTTP_GET ""
ENV START_SUB_REQ_HTTP_GET_SCHEDULED ""
ENV START_SUB_REQ_TAIL_FILE ""
ENV START_SUB_REQ_CLI_COMMAND_CONT ""
ENV START_SUB_REQ_RELAY ""

CMD ["ash","-c","env CONFIGFOLDER=./etc/ /app/steward\
    -ringBufferSize=$RING_BUFFER_SIZE\
    -socketFolder=$SOCKET_FOLDER\
    -tcpListener=$TCP_LISTENER\
    -httpListener=$HTTP_LISTENER\
    -databaseFolder=$DATABASE_FOLDER\
    -nodeName=$NODE_NAME\
    -brokerAddress=$BROKER_ADDRESS\
    -natsConnOptTimeout=$NATS_CONN_OPT_TIMEOUT\
    -natsConnectRetryInterval=$NATS_CONNECT_RETRY_INTERVAL\
    -natsReconnectJitter=$NATS_RECONNECT_JITTER\
    -natsReconnectJitterTLS=$NATS_RECONNECT_JITTER_TLS\
    -profilingPort=$PROFILING_PORT\
    -promHostAndPort=$PROM_HOST_AND_PORT\
    -defaultMessageTimeout=$DEFAULT_MESSAGE_TIMEOUT\
    -defaultMessageRetries=$DEFAULT_MESSAGE_RETRIES\
    -defaultMethodTimeout=$DEFAULT_METHOD_TIMEOUT\
    -subscribersDataFolder=$SUBSCRIBERS_DATA_FOLDER\
    -centralNodeName=$CENTRAL_NODE_NAME\
    -rootCAPath=$ROOT_CA_PATH\
    -nkeySeedFile=$NKEY_SEED_FILE\
    -exposeDataFolder=$EXPOSE_DATA_FOLDER\
    -errorMessageRetries=$ERROR_MESSAGE_RETRIES\
    -errorMessageTimeout=$ERROR_MESSAGE_TIMEOUT\
    -compression=$COMPRESSION\
    -serialization=$SERIALIZATION\
    -setBlockProfileRate=$SET_BLOCK_PROFILE_RATE\
    -enableSocket=$ENABLE_SOCKET\
    -enableTUI=$ENABLE_TUI\
    -enableSignatureCheck=$ENABLE_SIGNATURE_CHECK\
    -isCentralAuth=$IS_CENTRAL_AUTH\
    -enableDebug=$ENABLE_DEBUG\
    -startPubREQHello=$START_PUB_REQ_HELLO\
    -startSubREQErrorLog=$START_SUB_REQ_ERROR_LOG\
    -startSubREQHello=$START_SUB_REQ_HELLO\
    -startSubREQToFileAppend=$START_SUB_REQ_TO_FILE_APPEND\
    -startSubREQToFile=$START_SUB_REQ_TO_FILE\
    -startSubREQCopyFileFrom=$START_SUB_REQ_COPY_FILE_FROM\
    -startSubREQCopyFileTo=$START_SUB_REQ_COPY_FILE_TO\
    -startSubREQPing=$START_SUB_REQ_PING\
    -startSubREQPong=$START_SUB_REQ_PONG\
    -startSubREQCliCommand=$START_SUB_REQ_CLI_COMMAND\
    -startSubREQToConsole=$START_SUB_REQ_TO_CONSOLE\
    -startSubREQHttpGet=$START_SUB_REQ_HTTP_GET\
    -startSubREQHttpGetScheduled=$START_SUB_REQ_HTTP_GET_SCHEDULED\
    -startSubREQTailFile=$START_SUB_REQ_TAIL_FILE\
    -startSubREQCliCommandCont=$START_SUB_REQ_CLI_COMMAND_CONT\
    -startSubREQRelay=$START_SUB_REQ_RELAY\
    "]
