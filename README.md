# amqp-engine

amqp-engine is an integrated tool that simplifies AMQP functionality by exposing only the frequently used features and eliminating the need for users to focus on peripheral parameters.
Another part of it is an asynchronous message engine: when processing messages with AMQP, users can register a listening queue to handle messages similar to using [Gin](https://github.com/gin-gonic/gin).

## Get

    go get -u github.com/guoshaodong/amqp-engine

## Goals
1. Overall section
   - Supports connection and channel disconnection monitoring and reconnection.
   - Simple and easy to use while maintaining the flexibility of AMQP.
   - Suitable for current scenarios.
2. AMQP Section
   - Supports creating and deleting exchanges and queues freely
   - Supports binding and unbinding freely
   - Supports publishing different types of data, and returns the publishing results
   - Provides low-level AMQP applications when the scenario is more complex
3. Engine section
   - Autonomous reconnection
   - Error handling mechanism
   - Support for adding middleware
   - Support for grouping function, different groups can use different middleware
   - Support for binding functions of different data types
   - Users can freely control the subscription Ack mechanism

## Quick start
```sh
# assume the following codes in example.go file
$ cat example/engine.go
```

```go
package main

import (
	"github.com/guoshaodong/amqp-engine/engine"
)

type testStruct struct {
   Name string `json:"name"`
   Age  int    `json:"age"`
}

func main() {
   e := engine.New("amqp://username:password@127.0.0.1:5672/")

   e.Handle("queue", false, func(ctx *engine.Context) error {
      var s1 testStruct
      err := ctx.ShouldBindJson(&s1)
      if err != nil {
         return err
      }
      return nil
   })
   e.run()
}
```

## Example
See the [example](https://github.com/guoshaodong/amqp-engine/tree/main/example) subdirectory for simple engine executables.

## Engine design
![引擎设计图](https://github.com/guoshaodong/amqp-engine/blob/graphs/engine-design.jpg?inline=true)

## Future
1. Flexible support for type configuration, allowing developers to add types and a small amount of configuration to enable parsing of that type.
2. Engine functionality implementation is pluggable, as long as the underlying implementation has the main functions of Connection and Channel, it can be integrated into the engine.
3. Added "mustbind" for mandatory validation of bound fields, and "shouldbind" with tag:"binding" for enforced validation control.
4. Integration with opentelemetry for tracing of access links.
