# function-go

Generated with template for writing a [composition function][functions] in [Go][go].

This function uses [Go][go], [Docker][docker], and the [Crossplane CLI][cli] to
build functions.

## Test

Follow instructions in [Crossplane Docs](https://docs.crossplane.io/latest/guides/write-a-composition-function-in-go/#test-the-function-end-to-end).

```yaml
# functions.yaml
apiVersion: pkg.crossplane.io/v1
kind: Function
metadata:
  name: function-go
  annotations:
    # This tells crossplane render that your function is running locally.
    render.crossplane.io/runtime: Development
```

```shell
# Run function on http://localhost:9443
$ go run . --insecure --debug
```

In a separate terminal, run `crossplane render`.

```shell
crossplane render xr.yaml composition.yaml functions.yaml
```

## Develop

```shell
# Run code generation - see input/generate.go
$ go generate ./...

# Run tests - see fn_test.go
$ go test ./...

# Build the function's runtime image - see Dockerfile
$ docker build . --tag=runtime

# Build a function package - see package/crossplane.yaml
$ crossplane xpkg build -f package --embed-runtime-image=runtime
```

[functions]: https://docs.crossplane.io/latest/concepts/composition-functions
[go]: https://go.dev
[docker]: https://www.docker.com
[cli]: https://docs.crossplane.io/latest/cli
