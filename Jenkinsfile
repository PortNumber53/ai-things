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

    sh """
        set -x
        echo '${deploymentPath}'
        ssh ${sshConnection} mkdir -pv ${deploymentPath} || { echo "Failed to create releases directory"; exit 1; }
        rsync -rap --exclude=.git --exclude=.env.* --exclude=manager\\@tmp --exclude=manager/storage ./ ${sshConnection}:${deploymentPath} || { echo "rsync failed"; exit 1; }
        rsync -rap --exclude=.git ./.env.${sshConnection} ${sshConnection}:${deploymentPath}/.env || { echo "rsync failed"; exit 1; }
        ssh ${sshConnection} "cd ${deploymentPath} && ./deploy/deployment-script-${sshConnection}.sh ${deployBasePath} ${deploymentReleasePath} ${deploymentPath} ${timestamp}" || { echo "Deployment script execution failed"; exit 1; }
    """
}
