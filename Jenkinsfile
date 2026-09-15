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
        stage('Test') {
            steps {
                // Runs against the shared workstation Postgres/Redis, same as the
                // deployed app; a failure here stops the pipeline before Build/Deploy.
                sh '''
                    docker build --target backend -t tls-rest-test:ci .
                    docker run --rm \
                        --add-host=host.docker.internal:host-gateway \
                        -v /opt/workstation/.env:/opt/workstation/.env:ro \
                        -e DOTENV_PATH=/opt/workstation/.env \
                        tls-rest-test:ci \
                        go test ./test/...
                '''
            }
        }
        stage('Build') {
            steps {
                // APP_VERSION (git HEAD) -> Dockerfile ARG -> ENV APP_VERSION ->
                // os.Getenv in the app -> ?v=… asset cache-buster.
                // \$(...) is escaped so the shell — not Groovy — runs it.
                sh "docker build --build-arg APP_VERSION=\$(git rev-parse --short=8 HEAD) -t ${APP_NAME}:latest ."
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