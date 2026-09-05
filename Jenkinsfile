pipeline {
    agent any
    environment {
        APP_NAME = 'tls-rest'
    }
    stages {
        stage('Checkout') {
            steps {
                checkout scm
            }
        }
        stage('Build & Test') {
            steps {
                sh "docker build --no-cache -t ${APP_NAME}:latest ."
            }
        }
        stage('Deploy') {
            steps {
                sh 'docker compose up --force-recreate --no-build --detach'
            }
        }
    }
    post {
        always {
            sh "docker image prune -f"
        }
        failure {
            // Print logs to Jenkins output automatically if the container crashes
            sh "docker logs tls-rest --tail 50 || true"
        }
    }
}