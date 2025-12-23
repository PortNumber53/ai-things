pipeline {
    agent any

    environment {
        DEPLOY_PATH = '/deploy/ai-things/'
        TIMESTAMP = new Date().format("yyyyMMddHHmmss")
    }

    stages {
        stage('Checkout') {
            steps {
                // Checkout the code from your Git repository
                checkout scm
                sh 'git clean --dry-run'
                sh 'git clean -df'
            }
        }

        stage('Builds') {
            parallel {
                stage("Build Podcast") {
                    steps {
                        dir('podcast') {
                            sh 'npm install'
                            // sh 'npm run build'
                        }
                    }
                }
                stage('Prepare Manager') {
                    steps {
                        // Build Go manager and stage env files.
                        withCredentials([
                            file(credentialsId: 'ai-things-brain-env-prod-file', variable: 'ENV_FILE_BRAIN'),
                            file(credentialsId: 'ai-things-pinky-env-prod-file', variable: 'ENV_FILE_PINKY'),
                        ]) {
                            sh 'cd manager-go && go build -o manager ./cmd/manager'
                            sh 'cp --no-preserve=mode,ownership $ENV_FILE_BRAIN .env.brain'
                            sh 'cp --no-preserve=mode,ownership $ENV_FILE_PINKY .env.pinky'
                        }
                    }
                }
            }
        }

        stage('Deployments') {
            steps {
                script {
                    // Deploy to multiple hosts
                    def hosts = ['brain', 'pinky']
                    def ENV_FILES = [
                        brain: 'ai-things-brain-env-prod-file',
                        pinky: 'ai-things-pinky-env-prod-file',
                    ]
                    for (host in hosts) {
                        deployToHost(host, DEPLOY_PATH, ENV_FILES[host], TIMESTAMP)
                    }
                }
            }
        }
    }
}

def deployToHost(sshConnection, deployBasePath, envFile, timestamp) {
    def deploymentReleasePath = "${deployBasePath}releases/"
    def deploymentPath = "${deploymentReleasePath}${timestamp}"

    withCredentials([
        string(credentialsId: 'ai-things-database-url', variable: 'AI_THINGS_DATABASE_URL'),

        string(credentialsId: 'ai-things-rabbitmq-host', variable: 'AI_THINGS_RABBITMQ_HOST'),
        string(credentialsId: 'ai-things-rabbitmq-port', variable: 'AI_THINGS_RABBITMQ_PORT'),
        string(credentialsId: 'ai-things-rabbitmq-username', variable: 'AI_THINGS_RABBITMQ_USERNAME'),
        string(credentialsId: 'ai-things-rabbitmq-password', variable: 'AI_THINGS_RABBITMQ_PASSWORD'),
        string(credentialsId: 'ai-things-rabbitmq-vhost', variable: 'AI_THINGS_RABBITMQ_VHOST'),

        string(credentialsId: 'ai-things-base-output-folder', variable: 'AI_THINGS_BASE_OUTPUT_FOLDER'),
        string(credentialsId: 'ai-things-base-app-folder', variable: 'AI_THINGS_BASE_APP_FOLDER'),

        string(credentialsId: 'ai-things-subtitle-script', variable: 'AI_THINGS_SUBTITLE_SCRIPT'),
        string(credentialsId: 'ai-things-youtube-upload-script', variable: 'AI_THINGS_YOUTUBE_UPLOAD_SCRIPT'),
        string(credentialsId: 'ai-things-tiktok-upload-script', variable: 'AI_THINGS_TIKTOK_UPLOAD_SCRIPT'),

        string(credentialsId: 'ai-things-onnx-model', variable: 'AI_THINGS_ONNX_MODEL'),
        string(credentialsId: 'ai-things-tts-config-file', variable: 'AI_THINGS_TTS_CONFIG_FILE'),
        string(credentialsId: 'ai-things-tts-voice', variable: 'AI_THINGS_TTS_VOICE'),

        string(credentialsId: 'ai-things-ollama-hostname', variable: 'AI_THINGS_OLLAMA_HOSTNAME'),
        string(credentialsId: 'ai-things-ollama-port', variable: 'AI_THINGS_OLLAMA_PORT'),
        string(credentialsId: 'ai-things-ollama-model', variable: 'AI_THINGS_OLLAMA_MODEL'),

        string(credentialsId: 'ai-things-slack-app-id', variable: 'AI_THINGS_SLACK_APP_ID'),
        string(credentialsId: 'ai-things-slack-client-id', variable: 'AI_THINGS_SLACK_CLIENT_ID'),
        string(credentialsId: 'ai-things-slack-client-secret', variable: 'AI_THINGS_SLACK_CLIENT_SECRET'),
        string(credentialsId: 'ai-things-slack-signing-secret', variable: 'AI_THINGS_SLACK_SIGNING_SECRET'),
        string(credentialsId: 'ai-things-slack-port', variable: 'AI_THINGS_SLACK_PORT'),
        string(credentialsId: 'ai-things-slack-verification-token', variable: 'AI_THINGS_SLACK_VERIFICATION_TOKEN'),
        string(credentialsId: 'ai-things-slack-scopes', variable: 'AI_THINGS_SLACK_SCOPES'),
        string(credentialsId: 'ai-things-slack-redirect-url', variable: 'AI_THINGS_SLACK_REDIRECT_URL'),
    ]) {
        sh """
            set -euo pipefail
            set -x
            echo '${deploymentPath}'

            # Resolve per-host hostname (for queue routing).
            HOST_FQDN="\$(ssh ${sshConnection} 'hostname -f 2>/dev/null || hostname' | tr -d '\\r\\n')"

            # Render config.ini and install on host (no local temp file).
            set +x
            ssh ${sshConnection} 'sudo mkdir -p /etc/ai-things' || { echo "Failed to create /etc/ai-things"; exit 1; }
            cat <<EOF | ssh ${sshConnection} 'sudo tee /etc/ai-things/config.ini >/dev/null' || { echo "Failed to write /etc/ai-things/config.ini"; exit 1; }
[app]
hostname=\${HOST_FQDN}
env=production
base_output_folder=\${AI_THINGS_BASE_OUTPUT_FOLDER}
base_app_folder=\${AI_THINGS_BASE_APP_FOLDER}

[paths]
subtitle_script=\${AI_THINGS_SUBTITLE_SCRIPT}
youtube_upload_script=\${AI_THINGS_YOUTUBE_UPLOAD_SCRIPT}
tiktok_upload_script=\${AI_THINGS_TIKTOK_UPLOAD_SCRIPT}

[tts]
onnx_model=\${AI_THINGS_ONNX_MODEL}
config_file=\${AI_THINGS_TTS_CONFIG_FILE}
voice=\${AI_THINGS_TTS_VOICE}

[db]
url=\${AI_THINGS_DATABASE_URL}
database_url=\${AI_THINGS_DATABASE_URL}

[rabbitmq]
host=\${AI_THINGS_RABBITMQ_HOST}
port=\${AI_THINGS_RABBITMQ_PORT}
user=\${AI_THINGS_RABBITMQ_USERNAME}
password=\${AI_THINGS_RABBITMQ_PASSWORD}
vhost=\${AI_THINGS_RABBITMQ_VHOST}

[ollama]
hostname=\${AI_THINGS_OLLAMA_HOSTNAME}
port=\${AI_THINGS_OLLAMA_PORT}
model=\${AI_THINGS_OLLAMA_MODEL}

[slack]
app_id=\${AI_THINGS_SLACK_APP_ID}
client_id=\${AI_THINGS_SLACK_CLIENT_ID}
client_secret=\${AI_THINGS_SLACK_CLIENT_SECRET}
signing_secret=\${AI_THINGS_SLACK_SIGNING_SECRET}
port=\${AI_THINGS_SLACK_PORT}
verification_token=\${AI_THINGS_SLACK_VERIFICATION_TOKEN}
scopes=\${AI_THINGS_SLACK_SCOPES}
redirect_url=\${AI_THINGS_SLACK_REDIRECT_URL}
EOF
            ssh ${sshConnection} 'sudo chmod 600 /etc/ai-things/config.ini' || { echo "Failed to chmod /etc/ai-things/config.ini"; exit 1; }
            set -x

            ssh ${sshConnection} mkdir -pv ${deploymentPath} || { echo "Failed to create releases directory"; exit 1; }
            rsync -rap --exclude=.git --exclude=.env.* --exclude=manager\\@tmp --exclude=manager/storage ./ ${sshConnection}:${deploymentPath} || { echo "rsync failed"; exit 1; }
            rsync -rap --exclude=.git ./.env.${sshConnection} ${sshConnection}:${deploymentPath}/.env || { echo "rsync failed"; exit 1; }

            ssh ${sshConnection} "cd ${deploymentPath} && ./deploy/deployment-script-${sshConnection}.sh ${deployBasePath} ${deploymentReleasePath} ${deploymentPath} ${timestamp}" || { echo "Deployment script execution failed"; exit 1; }
        """
    }
}
