pipeline {
    agent any

    environment {
        // Define the credentials IDs for the .env file content
        ENV_FILES = [
            brain: 'ai-things-brain-env-prod-file',
            pinky: 'ai-things-pinky-env-prod-file',
            legion: 'ai-things-legion-env-prod-file',
            devbox: 'ai-things-devbox-env-prod-file'
        ]
        // Define the path to the Laravel app on the laptop
        DEPLOY_PATH = '/deploy/ai-things/'
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
                        dir('manager') {
                            sh 'npm install'
                            sh 'npm run build'
                        }
                    }
                }
                stage('Build API') {
                    steps {
                        dir('manager') {
                            // Run composer install with both secret files
                            withCredentials([
                                file(credentialsId: ENV_FILES.brain, variable: 'ENV_FILE_BRAIN'),
                                file(credentialsId: ENV_FILES.pinky, variable: 'ENV_FILE_PINKY'),
                                file(credentialsId: ENV_FILES.legion, variable: 'ENV_FILE_LEGION'),
                                file(credentialsId: ENV_FILES.devbox, variable: 'ENV_FILE_DEVBOX'),
                                ]) {
                                sh 'cp --no-preserve=mode,ownership $ENV_FILE_BRAIN .env.brain'
                                sh 'cp --no-preserve=mode,ownership $ENV_FILE_PINKY .env.pinky'
                                sh 'cp --no-preserve=mode,ownership $ENV_FILE_LEGION .env.legion'
                                sh 'cp --no-preserve=mode,ownership $ENV_FILE_DEVBOX .env.devbox'
                                sh 'composer install --no-ansi'
                            }
                        }
                    }
                }
                // Add more stages for other folders if needed
            }
        }

        stage('Deployments') {
            steps {
                script {
                    // Deploy to multiple hosts
                    def hosts = ['brain', 'pinky', 'legion', 'devbox'] // Add more hosts here if needed
                    for (host in hosts) {
                        deployToHost(host, DEPLOY_PATH, ENV_FILES[host])
                    }
                }
            }
        }

        stage('Apply Changes') {
            steps {
                // SSH into devbox and run Laravel migrations
                sh "ssh devbox 'cd ${env.LAPTOP_PATH}/manager && php artisan migrate --force'"
            }
        }
    }
}

def deployToHost(sshConnection, deployBasePath, envFile) {
    def timestamp = new Date().format("yyyyMMddHHmmss")
    def deploymentReleasePath = "${deployBasePath}/releases/"
    def deploymentPath = "${deploymentReleasePath}${timestamp}"

    sh """
        set -x
        echo '${deploymentPath}'
        ssh ${sshConnection} mkdir -pv ${deploymentPath} || { echo "Failed to create releases directory"; exit 1; }
        rsync -rap --exclude=.git --exclude=.env --exclude=manager/.env.* ./manager/ ${sshConnection}:${deploymentPath} || { echo "rsync failed"; exit 1; }
        rsync -rap --exclude=.git ./manager/.env.${sshConnection} ${sshConnection}:${deploymentPath}/.env || { echo "rsync failed"; exit 1; }
        rsync -rap --exclude=.git ./deploy/deployment-script.sh ${sshConnection}:${deploymentPath} || { echo "rsync failed"; exit 1; }
        rsync -rap --exclude=.git ./systemd/ ${sshConnection}:${deployBasePath}/systemd/ || { echo "rsync failed"; exit 1; }
        ssh ${sshConnection} "cd ${deploymentPath} && ./deployment-script.sh ${deployBasePath} ${deploymentReleasePath} ${deploymentPath} ${timestamp}" || { echo "Deployment script execution failed"; exit 1; }
    """
}
