# codebuild-run-build

Runs Codebuild Build and streams the output via Cloudwatch Logs.

Recommended for use with [aws-vault][] for authentication.

## Usage

```bash
$ aws-vault exec myprofile -- codebuild-run-build --project-name my-codebuild-project

Hello from Docker!
...
```

## Installation

```bash
go get github.com/buildkite/codebuild-run-build
```

[aws-vault]: https://github.com/99designs/aws-vault
