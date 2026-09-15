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
                // The Jenkins container has no Go toolchain (and gocv needs the
                // OpenCV native libs), so tests run inside the same base image the
                // Build stage compiles with. host.docker.internal is how the app
                // container itself reaches the shared workstation Postgres/Redis
                // (see docker-compose.yml's extra_hosts) — mirrored here so tests
                // hit the same instance the deployed app uses. A failure here stops
                // the pipeline before Build/Deploy.
                sh '''
                    docker run --rm \
                        --add-host=host.docker.internal:host-gateway \
                        -v "$WORKSPACE":/src -w /src \
                        -v /opt/workstation/.env:/opt/workstation/.env:ro \
                        -v go-mod-cache:/root/go/pkg/mod \
                        -e DOTENV_PATH=/opt/workstation/.env \
                        -e CGO_ENABLED=1 \
                        ghcr.io/hybridgroup/opencv:4.13.0 \
                        bash -c "apt-get update -qq && apt-get install -y -qq --no-install-recommends libpcap-dev >/dev/null && go test ./test/..."
                '''
            }
        }
        stage('Build') {
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