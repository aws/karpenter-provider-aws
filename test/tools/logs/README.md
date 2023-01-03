This CLI tool exports CloudWatch Logs for a specific test ID to S3 and then downloads the S3 logs to stdout so that you can grep and manipulate the logs on your local computer.

## Usage:

```bash
AWS_PROFILE=karpenter-ci go run main.go upgrade-suiteqwhlx | sort | less
```
