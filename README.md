# gpugen

Translates Go functions marked `//gpu:kernel` into WGSL compute shaders.

```go
//go:generate go run github.com/dstuden/gpugen/cmd/gpugen

//gpu:kernel
func Saxpy(id uint32, a float32, x []float32, y []float32) {
    y[id] = a*x[id] + y[id]
}
```

## Example

```sh
go run ./examples/blur -in examples/blur/testdata/sponza.jpg -sigma 4
go test ./...
```
