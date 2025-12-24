# Project Notes

## Jenkins credentials used to render `/etc/ai-things/config.ini`

These values are pulled from Jenkins Credentials during deployment and are **not** committed to this repo.

### Database
- **ai-things-database-url**

### RabbitMQ
- **ai-things-rabbitmq-host**
- **ai-things-rabbitmq-port**
- **ai-things-rabbitmq-username**
- **ai-things-rabbitmq-password**
- **ai-things-rabbitmq-vhost**

### Base folders
- **ai-things-base-output-folder**
- **ai-things-base-app-folder**

### Script paths
- **ai-things-subtitle-script**
- **ai-things-youtube-upload-script**
- **ai-things-tiktok-upload-script**

### TTS
- **ai-things-onnx-model**
- **ai-things-tts-config-file**
- **ai-things-tts-voice**

### Ollama
- **ai-things-ollama-hostname**
- **ai-things-ollama-port**
- **ai-things-ollama-model**

### Slack
- **ai-things-slack-app-id**
- **ai-things-slack-client-id**
- **ai-things-slack-client-secret**
- **ai-things-slack-signing-secret**
- **ai-things-slack-port**
- **ai-things-slack-verification-token**
- **ai-things-slack-scopes**
- **ai-things-slack-redirect-url**

### EXTRA_ENV
- **ai-things-portnumber53-api-key**


## Deployment behavior

- `app.hostname` is resolved on the target host via `hostname -f` (fallback to `hostname`).
- `config.ini` is written to `/etc/ai-things/config.ini` with mode `0600` (requires passwordless `sudo` for the deployment SSH user).


