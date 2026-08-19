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
                sh 'docker build -t ${APP_NAME}:latest .'
            }
        }
        stage('Deploy') {
            when {
                branch 'main'
            }
            steps {
                sh 'docker compose -f ${COMPOSE_FILE} down --remove-orphans || true'
                sh 'docker compose -f ${COMPOSE_FILE} up -d'
            }
        }
    }
    post {
        always {
            sh 'docker image prune -f'
        }
    }
}