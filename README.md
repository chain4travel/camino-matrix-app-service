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
For example, see [example/config/synapse/camino.yaml](example/config/synapse/camino.yaml)

### App-service config
App-service itself has its own config. Docker container expects this config to be at `/camino-matrix-app-service/camino-matrix-app-service.yaml`.
For example, see [example/config/camino-matrix-app-service.yaml](example/config/camino-matrix-app-service.yaml)

## License

The Camino Matrix App Service is licensed under the terms of the [Camino Messenger License](LICENSE.md).

## Data Protection

Please take note of the [Camino Messenger Data Protection Guidelines](DATA_PROTECTION.md).
