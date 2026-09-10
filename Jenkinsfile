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
                // APP_VERSION (git HEAD) -> Dockerfile ARG -> ENV APP_VERSION ->
                // os.Getenv in the app -> ?v=… asset cache-buster.
                // \$(...) is escaped so the shell — not Groovy — runs it.
                sh "docker build --no-cache --build-arg APP_VERSION=\$(git rev-parse --short=8 HEAD) -t ${APP_NAME}:latest ."
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