pipeline {
    agent any
    environment {
        APP_NAME = 'tls-rest'
        // Separate repo (not part of this one) — the Dockerfile's "rig" stage
        // builds whatever lands in ./rust-callmesh-rig at this exact path.
        // Private, so the Fetch rig stage below authenticates via a Jenkins
        // credential — see that stage's own comment for what to create.
        RIG_REPO_URL = 'https://github.com/ws117z5/rust-callmesh-rig.git'
    }
    stages {
        stage('Checkout') {
            steps {
                checkout scm
            }
        }
        stage('Fetch rig') {
            steps {
                // Always re-fetch so a build picks up the rig repo's latest
                // commit, same as checkout scm does for this repo itself.
                // Private repo -> plain `git clone` has no TTY to prompt for a
                // username in Jenkins and fails outright ("could not read
                // Username ... No such device or address"); a Jenkins
                // "Username with password" credential (password = a GitHub
                // PAT with repo scope) gets embedded into the URL instead —
                // withCredentials also auto-masks it in console output, which
                // a value pulled from a plain file wouldn't get for free.
                // Create it once: Manage Jenkins -> Credentials -> add
                // "Username with password", ID "rig-repo-github" (or change
                // the ID below to match whatever you actually name it).
                withCredentials([usernamePassword(credentialsId: 'rig-repo-github', usernameVariable: 'RIG_GIT_USER', passwordVariable: 'RIG_GIT_TOKEN')]) {
                    sh '''
                        rm -rf rust-callmesh-rig
                        git clone --depth 1 "https://${RIG_GIT_USER}:${RIG_GIT_TOKEN}@${RIG_REPO_URL#https://}" rust-callmesh-rig
                    '''
                }
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