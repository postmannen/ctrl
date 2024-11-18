# Nats timeouts

The various timeouts for the NATS messages can be controlled via an .env file or flags.

If the network media is a high latency like satellite links, it will make sense to adjust the client timeout to reflect the latency

```text
  -natsConnOptTimeout int
        default nats client conn timeout in seconds (default 20)
```

The interval in seconds the nats client should try to reconnect to the nats-server if the connection is lost.

```text
  -natsConnectRetryInterval int
        default nats retry connect interval in seconds. (default 10)
```

Jitter values.

```text
  -natsReconnectJitter int
        default nats ReconnectJitter interval in milliseconds. (default 100)
  -natsReconnectJitterTLS int
        default nats ReconnectJitterTLS interval in seconds. (default 5)
```