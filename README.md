# aws-mockery [![GoDoc](https://godoc.org/github.com/codeactual/aws-mockery?status.svg)](https://godoc.org/github.com/codeactual/aws-mockery) [![Go Report Card](https://goreportcard.com/badge/github.com/codeactual/aws-mockery)](https://goreportcard.com/report/github.com/codeactual/aws-mockery) [![Build Status](https://travis-ci.org/codeactual/aws-mockery.png)](https://travis-ci.org/codeactual/aws-mockery)

aws-mockery is a program which uses the [mockery](https://github.com/vektra/mockery) API to generate implementations of selected [aws-sdk-go](https://github.com/aws/aws-sdk-go) service interfaces.

Currently only [aws-sdk-go](https://github.com/aws/aws-sdk-go) version `1.x` is supported.

## Use Case

Asserting that dependent code interacts with the service APIs as expected, but without the cost (and feedback fidelity) of making real requests to the service.

# Usage

> To install: `go get -v github.com/codeactual/aws-mockery/cmd/aws-mockery`

## Examples

> Display help:

```bash
aws-mockery --help
```

> Output mock implementations for KMS, Route53, and SNS:

```bash
aws-mockery --out-dir /path/to/mocks --sdk-dir /path/to/github.com/aws/aws-sdk-go --service=kms,route53,sns
```

- `--service` expects a comma-separated list of identifiers, e.g. `ec2`, which must be a directory name from the SDK's [service](https://github.com/aws/aws-sdk-go/tree/master/service) directory.

# License

[Mozilla Public License Version 2.0](https://www.mozilla.org/en-US/MPL/2.0/) ([About](https://www.mozilla.org/en-US/MPL/), [FAQ](https://www.mozilla.org/en-US/MPL/2.0/FAQ/))

*(Exported from a private monorepo with [transplant](https://github.com/codeactual/transplant).)*
