//go:build js && wasm

package main

import (
	core "github.com/Gk0Wk/openapi-thrift"
	"syscall/js"
)

func main() {
	dispatch := js.FuncOf(func(_ js.Value, args []js.Value) interface{} {
		if len(args) != 1 || args[0].Type() != js.TypeString {
			return `{"error":{"name":"Error","message":"core expects one JSON string"}}`
		}
		return string(core.DispatchJSON([]byte(args[0].String())))
	})
	js.Global().Set("__openapiThriftDispatch", dispatch)
	select {}
}
