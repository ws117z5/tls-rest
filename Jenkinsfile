pipeline {
    agent any
    environment {
        APP_NAME = 'tls-rest'
        COMPOSE_FILE = 'docker-compose.yml'
    }
    stages {
        stage('Checkout') {
            steps {
                checkout scm
            }
        }
        stage('Build & Test') {
            steps {
                sh 'docker build --no-cache -t ${APP_NAME}:latest .'
            }
        }
        stage('Deploy') {
            steps {
                sh 'docker compose -f ${COMPOSE_FILE} down --remove-orphans || true'
                sh 'docker compose -f ${COMPOSE_FILE} up -d --force-recreate --renew-anon-volumes --no-build'
            }
        }
    }
    post {
        always {
            sh 'docker image prune -f'
        }
    }
}