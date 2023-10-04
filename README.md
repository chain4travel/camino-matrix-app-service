## Docker
Build docker with:
```
docker build -t c4tplatform/camino-synapse-app-service .
```

## Configs
### Synapse app-service registration
Synapse server must be provided with synapse app-service registration config at `files/matrix/.synapse/camino.yaml`.

See `camino.yaml.example`:
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
App-service itself has its own config. Docker container expects this config to be at `/camino-synapse-app-service/camino-synapse-app-service.yaml`.

See `camino-synapse-app-service.yaml.example`:
```yaml
cashout_period: 30s
camino_node_host: http://localhost:19651
matrix_host: http://localhost:8008
http_port: 5000
access_token: wfghWEGh3wgWHEf3478sHFWE
matrix_access_token: ugw8243igya57aaABGFfgeyu
log_level: debug # debug or info
db_path: db.db
db_name: camino_synapse_app_service
migrations_path: file://./migrations # schema is mandatory!
```