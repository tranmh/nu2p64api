// The safety net of nu2p64api, written by aqaa from what its QA run found: unit tests with coverage and its
// floor, integration tests against the databases they need, end-to-end tests, and lint, SAST and dependency
// checks (report only: a finding marks the stage unstable until it is triaged).
// Needs the plugins Pipeline, Docker Pipeline, JUnit, Coverage, Timestamper and Workspace Cleanup;
// integration stages run on an agent labelled 'docker'.
pipeline {
  agent none
  options {
    timeout(time: 90, unit: 'MINUTES')
    timestamps()
    buildDiscarder(logRotator(numToKeepStr: '30', artifactNumToKeepStr: '10'))
    disableConcurrentBuilds(abortPrevious: true)
  }
  environment {
    CI = 'true'
  }
  stages {
    stage('Tests') {
      parallel {
        stage('Unit: go') {
          agent { docker { image 'golang:1.20' } }
          options { timeout(time: 30, unit: 'MINUTES') }
          steps {
            dir('.') {
              sh '''
                set -eu
                mkdir -p test-reports
                go mod download
                go run gotest.tools/gotestsum@v1.13.0 --format testname --junitfile test-reports/unit.xml -- -count=1 -covermode=atomic -coverpkg=./... -coverprofile=test-reports/coverage.out ./...
              '''
            }
          }
          post {
            always {
              junit allowEmptyResults: true, testResults: 'test-reports/*.xml'
              recordCoverage id: 'coverage-go', name: 'Coverage go', tools: [[parser: 'GO_COV', pattern: 'test-reports/coverage.out']], sourceDirectories: [[path: '.']], qualityGates: [[threshold: 12.0, metric: 'LINE', baseline: 'PROJECT', criticality: 'FAILURE']]
              archiveArtifacts allowEmptyArchive: true, artifacts: 'test-reports/**'
            }
            cleanup {
              cleanWs deleteDirs: true, notFailBuild: true
            }
          }
        }
        stage('Integration: go') {
          agent { label 'docker' }
          options { timeout(time: 45, unit: 'MINUTES') }
          steps {
            script {
              docker.image('postgres:16-alpine').withRun('-e POSTGRES_USER=aqaa -e POSTGRES_PASSWORD=aqaa -e POSTGRES_DB=aqaa') { postgres ->
                docker.image('postgres:16-alpine').inside("--link ${postgres.id}:postgres") {
                  sh 'until pg_isready -h postgres -U aqaa -d aqaa; do sleep 1; done'
                }
                docker.image('golang:1.20').inside("--link ${postgres.id}:postgres -e AQAA_PG_URL=postgres://aqaa:aqaa@postgres:5432/aqaa?sslmode=disable -e DATABASE_URL=postgres://aqaa:aqaa@postgres:5432/aqaa?sslmode=disable -e 'AQAA_PG_CONNECTION_STRING=Host=postgres;Port=5432;Database=aqaa;Username=aqaa;Password=aqaa'") {
                  dir('.') {
                    sh '''
                      set -eu
                      mkdir -p test-reports
                      go mod download
                      RUN_INTEGRATION=1 INTEGRATION=1 go run gotest.tools/gotestsum@v1.13.0 --format testname --junitfile test-reports/integration.xml -- -count=1 -tags integration -run Integration ./...
                    '''
                  }
                }
              }
            }
          }
          post {
            always {
              junit allowEmptyResults: true, testResults: 'test-reports/*.xml'
              archiveArtifacts allowEmptyArchive: true, artifacts: 'test-reports/**'
            }
            cleanup {
              cleanWs deleteDirs: true, notFailBuild: true
            }
          }
        }
      }
    }
    stage('Checks') {
      parallel {
        stage('Checks: go') {
          agent { docker { image 'golang:1.20' } }
          options { timeout(time: 30, unit: 'MINUTES') }
          steps {
            dir('.') {
              sh '''
                set -eu
                mkdir -p test-reports
                go mod download
              '''
              catchError(buildResult: 'SUCCESS', stageResult: 'UNSTABLE', message: 'go vet found issues') {
                sh '''
                  set -eu
                  go vet ./...
                '''
              }
              catchError(buildResult: 'SUCCESS', stageResult: 'UNSTABLE', message: 'golangci-lint found issues') {
                sh '''
                  set -eu
                  go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run --output.text.path stdout --output.sarif.path test-reports/golangci.sarif ./...
                '''
              }
              catchError(buildResult: 'SUCCESS', stageResult: 'UNSTABLE', message: 'govulncheck found issues') {
                sh '''
                  set -eu
                  go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
                '''
              }
            }
          }
          post {
            always { archiveArtifacts allowEmptyArchive: true, artifacts: 'test-reports/**' }
            cleanup { cleanWs deleteDirs: true, notFailBuild: true }
          }
        }
      }
    }
  }
}
