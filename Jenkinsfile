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
            }
        }

        stage('Builds') {
            parallel {
                stage("Build Frontend") {
                    steps {
                        dir('frontend') {
                            sh 'npm install'
                            // sh 'npm run build'
                        }
                    }
                }
                stage('Prepare Manager') {
                    steps {
                        // Run composer install with all secret files
                        withCredentials([
                            file(credentialsId: 'ai-things-brain-env-prod-file', variable: 'ENV_FILE_BRAIN'),
                            file(credentialsId: 'ai-things-pinky-env-prod-file', variable: 'ENV_FILE_PINKY'),
                            file(credentialsId: 'ai-things-legion-env-prod-file', variable: 'ENV_FILE_LEGION'),
                            file(credentialsId: 'ai-things-devbox-env-prod-file', variable: 'ENV_FILE_DEVBOX'),
                        ]) {
                            sh 'cp --no-preserve=mode,ownership $ENV_FILE_BRAIN .env.brain'
                            sh 'cp --no-preserve=mode,ownership $ENV_FILE_PINKY .env.pinky'
                            sh 'cp --no-preserve=mode,ownership $ENV_FILE_LEGION .env.legion'
                            sh 'cp --no-preserve=mode,ownership $ENV_FILE_DEVBOX .env.devbox'
                        }
                    }
                }
            }
        }

        stage('Deployments') {
            steps {
                script {
                    // Deploy to multiple hosts
                    def hosts = ['brain', 'pinky', 'devbox'] //'legion',
                    def ENV_FILES = [
                        brain: 'ai-things-brain-env-prod-file',
                        pinky: 'ai-things-pinky-env-prod-file',
                        legion: 'ai-things-legion-env-prod-file',
                        devbox: 'ai-things-devbox-env-prod-file'
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
