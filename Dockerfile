# build stage
FROM golang:1.22-alpine AS build-env
RUN apk --no-cache add build-base git gcc

RUN mkdir -p /build
COPY ./ /build/

WORKDIR /build/cmd/ctrl/
RUN go version
RUN go build -o ctrl

# final stage
FROM alpine

RUN apk update && apk add curl && apk add nmap

WORKDIR /app
COPY --from=build-env /build/cmd/ctrl/ctrl /app/

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
ENV REQ_KEYS_REQUEST_UPDATE_INTERVAL "60"
ENV REQ_ACL_REQUEST_UPDATE_INTERVAL "60"
ENV PROFILING_PORT ""
ENV PROM_HOST_AND_PORT "127.0.0.1:2111"
ENV DEFAULT_MESSAGE_TIMEOUT 10
ENV DEFAULT_MESSAGE_RETRIES 3
ENV DEFAULT_METHOD_TIMEOUT 10
ENV SUBSCRIBERS_DATA_FOLDER "./var"
ENV CENTRAL_NODE_NAME "central"
ENV ROOT_CA_PATH ""
ENV NKEY_SEED_FILE ""
ENV NKEY_SEED ""
ENV EXPOSE_DATA_FOLDER "127.0.0.1:8090"
ENV ERROR_MESSAGE_RETRIES 3
ENV ERROR_MESSAGE_TIMEOUT 10
ENV COMPRESSION ""
ENV SERIALIZATION ""
ENV SET_BLOCK_PROFILE_RATE "0"
ENV ENABLE_SOCKET "1"
ENV ENABLE_SIGNATURE_CHECK "0"
ENV ENABLE_ACL_CHECK "0"
ENV IS_CENTRAL_AUTH "0"
ENV ENABLE_DEBUG "0"
ENV KEEP_PUBLISHERS_ALIVE_FOR "10"

ENV START_PUB_REQ_HELLO 60

ENV ENABLE_KEY_UPDATES "1"
ENV ENABLE_ACL_UPDATES "1"
ENV IS_CENTRAL_ERROR_LOGGER "0"
ENV START_SUB_REQ_HELLO "1"
ENV START_SUB_REQ_TO_FILE_APPEND "1"
ENV START_SUB_REQ_TO_FILE "1"
ENV START_SUB_REQ_TO_FILE_NACK "1"
ENV START_SUB_REQ_COPY_SRC "1"
ENV START_SUB_REQ_COPY_DST "1"
ENV START_SUB_REQ_CLI_COMMAND "1"
ENV START_SUB_REQ_TO_CONSOLE "1"
ENV START_SUB_REQ_HTTP_GET "1"
ENV START_SUB_REQ_HTTP_GET_SCHEDULED "1"
ENV START_SUB_REQ_TAIL_FILE "1"
ENV START_SUB_REQ_CLI_COMMAND_CONT "1"

CMD ["ash","-c","env CONFIGFOLDER=./etc/ /app/ctrl\
    -socketFolder=${SOCKET_FOLDER}\
    -tcpListener=${TCP_LISTENER}\
    -httpListener=${HTTP_LISTENER}\
    -databaseFolder=${DATABASE_FOLDER}\
    -nodeName=${NODE_NAME}\
    -brokerAddress=${BROKER_ADDRESS}\
    -natsConnOptTimeout=${NATS_CONN_OPT_TIMEOUT}\
    -natsConnectRetryInterval=${NATS_CONNECT_RETRY_INTERVAL}\
    -natsReconnectJitter=${NATS_RECONNECT_JITTER}\
    -natsReconnectJitterTLS=${NATS_RECONNECT_JITTER_TLS}\
    -REQKeysRequestUpdateInterval=${REQ_KEYS_REQUEST_UPDATE_INTERVAL}\
    -REQAclRequestUpdateInterval=${REQ_ACL_REQUEST_UPDATE_INTERVAL}\
    -profilingPort=${PROFILING_PORT}\
    -promHostAndPort=${PROM_HOST_AND_PORT}\
    -defaultMessageTimeout=${DEFAULT_MESSAGE_TIMEOUT}\
    -defaultMessageRetries=${DEFAULT_MESSAGE_RETRIES}\
    -defaultMethodTimeout=${DEFAULT_METHOD_TIMEOUT}\
    -subscribersDataFolder=${SUBSCRIBERS_DATA_FOLDER}\
    -centralNodeName=${CENTRAL_NODE_NAME}\
    -rootCAPath=${ROOT_CA_PATH}\
    -nkeySeedFile=${NKEY_SEED_FILE}\
    -nkeySeed=${NKEY_SEED}\
    -exposeDataFolder=${EXPOSE_DATA_FOLDER}\
    -errorMessageRetries=${ERROR_MESSAGE_RETRIES}\
    -errorMessageTimeout=${ERROR_MESSAGE_TIMEOUT}\
    -compression=${COMPRESSION}\
    -serialization=${SERIALIZATION}\
    -setBlockProfileRate=${SET_BLOCK_PROFILE_RATE}\
    -enableSocket=${ENABLE_SOCKET}\
    -enableSignatureCheck=${ENABLE_SIGNATURE_CHECK}\
    -enableAclCheck=${ENABLE_ACL_CHECK}\
    -isCentralAuth=${IS_CENTRAL_AUTH}\
    -enableDebug=${ENABLE_DEBUG}\
    -keepPublishersAliveFor=${KEEP_PUBLISHERS_ALIVE_FOR}\
    -startPubREQHello=${START_PUB_REQ_HELLO}\
    -EnableKeyUpdates=${ENABLE_KEY_UPDATES}\
    -EnableAclUpdates=${ENABLE_ACL_UPDATES}\
    -isCentralErrorLogger=${IS_CENTRAL_ERROR_LOGGER}\
    -startSubREQHello=${START_SUB_REQ_HELLO}\
    -startSubREQToFileAppend=${START_SUB_REQ_TO_FILE_APPEND}\
    -startSubREQToFile=${START_SUB_REQ_TO_FILE}\
    -startSubREQCopySrc=${START_SUB_REQ_COPY_SRC}\
    -startSubREQCopyDst=${START_SUB_REQ_COPY_DST}\
    -startSubREQToFileNACK=${START_SUB_REQ_TO_FILE_NACK}\
    -startSubREQCliCommand=${START_SUB_REQ_CLI_COMMAND}\
    -startSubREQToConsole=${START_SUB_REQ_TO_CONSOLE}\
    -startSubREQHttpGet=${START_SUB_REQ_HTTP_GET}\
    -startSubREQHttpGetScheduled=${START_SUB_REQ_HTTP_GET_SCHEDULED}\
    -startSubREQTailFile=${START_SUB_REQ_TAIL_FILE}\
    -startSubREQCliCommandCont=${START_SUB_REQ_CLI_COMMAND_CONT}\
    "]
