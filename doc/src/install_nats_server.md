# NATS Server install

ctrl uses NATS as the messagaging backbone. The following text will describe how to quickly get up and running with a minimal NATS setup. For full details of what you can do with nats-server, check out the official docs at [https://docs.nats.io/running-a-nats-service/introduction/installation](https://docs.nats.io/running-a-nats-service/introduction/installation)

## NKEY

NATS uses ED25519 based keys called NKEY's for Authentication and Authorization. The keys are created by a tool called **nk**. The instructions for how to install it are found here [https://docs.nats.io/using-nats/nats-tools/nk](https://docs.nats.io/using-nats/nats-tools/nk).

The private key are called **seed**, and the public key are called **user**.

To create the keys run the following command after the nk tool is installed.

```bash
nk -gen user -pubout
```

The tool will print out two new keys. Where the private Seed starts with the letter ` S `, and the public User key starts with the letter ` U `.

The private Seed key are used with each ctrl instance, and are referenced as an ENV, flag, or via file.

The public User key are used in the nats-server config file for Authentication, to define access lists for what Nats Subjects the ctrl instances should be allowed to send to, or receive from.

## Install the NATS Server

For this example we use docker compose to start the NATS server.

On your local computer create a folder to hold the NATS docker compose, and configuration files.

```bash
mkdir nats && cd nats
```

create the docker compose file called `nats.yaml`, with the following content.

```yaml
version: "3"

services:
  nats:
    build: .
    image: nats:latest
    # -js enables jetstram on the nats server.
    command: "-c /app/nats-server.conf -D -js"
    restart: always
    ports:
      - "4222:4222"
    volumes:
      - ./nats.conf:/app/nats-server.conf
    logging:
        driver: "json-file"
        options:
            max-size: "10m"
            max-file: "10"
```

In the same directory create the nats-server.conf file, with the following content. Replace the placeholders for the user keys in the acl with the user keys you created earlier.

```json
port: 4222

ACL = {
    publish: {
            allow: [">"]
    }
    subscribe: {
            allow: [">"]
    }
}

authorization: {
    timeout: "30s"
    users = [
        {
            # github
            nkey: <REPLACE WITH github user key here>
            permissions: $ACL
        },
        {
            # node1
            nkey: <REPLACE WITH seed user key here>
            permissions: $ACL
        },
    ]
}
```

## Firewall openings for NATS Server

You will need to open your firewall for inbound `tcp/4222` from the internet.

You can find your public ip address here [https://ipv4.jsonip.com/](https://ipv4.jsonip.com/).

## Other

More details like how to use certificates to encrypt the communication can be found in the official nats docs, [https://docs.nats.io/](https://docs.nats.io/).
