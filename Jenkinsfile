pipeline {
    agent any

    environment {
        // Define the credentials ID for the .env file content
        ENV_FILE_CREDENTIALS_ID = 'ai-things-env-file-prod'
        // Define the path to the Laravel app on the laptop
        LAPTOP_PATH = '/deploy/ai-things/'
    }

    stages {
        stage('Checkout') {
            steps {
                // Checkout the code from your Git repository
                checkout scm
            }
        }

        stage('Build') {
            parallel {
                stage('Install Composer Dependencies') {
                    steps {
                        // Install Composer dependencies in the manager folder
                        dir('manager') {
                            sh 'composer install'
                        }
                    }
                }
                stage('Run NPM Dev') {
                    steps {
                        // Run npm run dev in the manager folder
                        dir('manager') {
                            sh 'npm install'
                            sh 'npm run build'
                        }
                    }
                }
            }
        }

        stage('Deploy') {
            steps {
                // Retrieve the .env file content from Jenkins credentials
                withCredentials([file(credentialsId: ENV_FILE_CREDENTIALS_ID, variable: 'ENV_FILE')]) {
                    // Write the .env file content to a file
                    writeFile file: 'manager/.env', text: sh(script: 'cat $ENV_FILE', returnStdout: true).trim()
                }

                // Sync Laravel app files to the laptop using rsync
                sh "rsync -avz --delete ./ devbox:${env.LAPTOP_PATH}"
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
