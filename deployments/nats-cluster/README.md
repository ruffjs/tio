# tio NATS Cluster Deployment

Three-node tio cluster with embedded NATS clustering and a shared MySQL database.
All nodes use JetStream replicas (R=3) for MQTT streams and presence KV.

## Architecture

```
                    ┌──────────┐
                    │  MySQL   │
                    └────┬─────┘
               ┌─────────┼─────────┐
          ┌────┴────┐ ┌──┴───┐ ┌──┴────┐
          │tio-node-1├─┤tio-2 ├─┤tio-3 │
          │ :4222    │ │:4223 │ │:4224  │
          │ :1883    │ │:1884 │ │:1885  │
          │ :9000    │ │:9001 │ │:9002  │
          └──────────┘ └──────┘ └───────┘
              NATS cluster routes (port 6222-6224)
```

| Port  | Service      | Node 1 | Node 2 | Node 3 |
|-------|--------------|--------|--------|--------|
| NATS  | Client       | 4222   | 4223   | 4224   |
| NATS  | Cluster      | 6222   | 6223   | 6224   |
| MQTT  | TCP          | 1883   | 1884   | 1885   |
| MQTT  | WS           | 8083   | 8084   | 8085   |
| HTTP  | REST API     | 9000   | 9001   | 9002   |

## Prerequisites

- Docker and Docker Compose v2
- tio Docker image built: `docker build -t tio:latest -f build/docker/Dockerfile .`

## Start / Stop

```bash
cd deployments/nats-cluster

docker compose up -d
docker compose down          # stop and remove containers
docker compose down -v       # also remove volumes (destroys data)
```

## Verify Cluster Health

Check that all three NATS nodes see each other:

```bash
# NATS monitoring (each node exposes its own varz)
curl -s http://localhost:9000/api/v1/config | python3 -m json.tool
docker compose logs tio-node-1 | grep -i "route"
docker compose logs tio-node-2 | grep -i "route"
docker compose logs tio-node-3 | grep -i "route"
```

## TLS (Optional)

To enable TLS for client connections, add cert paths to each node config:

```yaml
connector:
  nats:
    server:
      clientTls:
        certFile: /app/certs/server-cert.pem
        keyFile: /app/certs/server-key.pem
        caFile: /app/certs/ca.pem
```

Mount the certs directory in docker-compose.yml:

```yaml
volumes:
  - ./certs:/app/certs:ro
```

## Testing

Run the in-process cluster acceptance tests:

```bash
go test -tags=integration ./integration_tests/cluster -run 'Cluster' -count=1 -v
```

Manual smoke test with nats client:

```bash
# subscribe on node 2
nats sub --server nats://localhost:4223 "test.topic"

# publish from node 1
nats pub --server nats://localhost:4222 "test.topic" "hello cluster"
```
