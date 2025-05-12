## Docker
Build docker with:
```
docker build -t c4tplatform/camino-matrix-app-service .
```

## Configs
# Camino-conduit app-service registration
Camino-conduit will register app-service on start-up

### Synapse app-service registration
Synapse server must be provided with synapse app-service registration config at `files/matrix/.synapse/camino.yaml`.

See `example/synapse/camino.yaml`:
```yaml
id: camino
url: http://host.docker.internal:5000
as_token: wfghWEGh3wgWHEf3478sHFWE
hs_token: ugw8243igya57aaABGFfgeyu
sender_localpart: camino
namespaces:
  users:
    - exclusive: false
      regex: ".*"
  aliases:
    - exclusive: false
      regex: ".*"
  rooms:
    - exclusive: false
      regex: ".*"
```

### App-service config
App-service itself has its own config. Docker container expects this config to be at `/camino-matrix-app-service/camino-matrix-app-service.yaml`.

See `example/camino-matrix-app-service.yaml`:
```yaml
cash_in_period: 30s # period for cash in
camino_node_host: http://localhost:19651
http_port: 5000
matrix_access_token: ugw8243igya57aaABGFfgeyu
log_level: debug # debug or info
db_path: db.db
db_name: camino_synapse_app_service
cm_account_contract_address: 0x4626cb544230e4d13fb72950501ff91740116a0a # cm account that will be network fee recipient
network_fee_recipient_key: <private key in hex format>
min_duration_until_expiration: 30s # minimum valid duration until expiration expiration for received cheques
```