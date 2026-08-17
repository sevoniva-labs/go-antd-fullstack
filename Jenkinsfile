pipeline {
  agent { label 'domestic-offline' }
  options {
    disableConcurrentBuilds(abortPrevious: true)
    timestamps()
  }
  environment {
    GOPROXY = 'https://goproxy.cn'
    GOSUMDB = 'sum.golang.google.cn'
    NPM_REGISTRY = 'https://registry.npmmirror.com'
    npm_config_registry = 'https://registry.npmmirror.com'
  }
  stages {
    stage('Policy') {
      steps { sh 'make ci-policy' }
    }
    stage('Backend') {
      steps { sh 'make ci-go' }
    }
    stage('Frontend') {
      steps { sh 'make ci-web' }
    }
    stage('Deploy') {
      steps { sh 'make ci-deploy' }
    }
  }
}
