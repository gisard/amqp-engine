# amqp-engine

amqp-engine 是一个集成化的工具，在 AMQP 功能的基础上做了更多的简化，只暴露用户使用比较频繁的功能，也使得用户不再关注那些旁枝末节的参数。
另一部分是异步消息引擎：在对 AMQP 进行消息处理时，用户可以像使用 [Gin](https://github.com/gin-gonic/gin) 一样，通过注册监听的队列从而对消息进行处理

## Get

    go get -u github.com/guoshaodong/amqp-engine

## Goals
1. 整体部分
    - 支持 connection、channel 断线监听、断线重连
    - 在保持 AMQP 的灵活性基础上，简单易用
    - 适配当前场景
2. AMQP 部分
    - 支持自由创建、删除 exchange 和 queue
    - 支持自由绑定和解绑
    - 支持发布不同类型的数据，并返回发布结果
    - 场景比较复杂时，能够提供底层 amqp 应用
3. Engine 部分
    - 自主断线重连
    - 拥有错误机制
    - 支持增加中间件
    - 支持分组功能，不同分组可以使用不同中间件
    - 支持不同数据类型的绑定功能
    - 用户能够自由控制订阅 Ack 机制

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
1. 灵活支持类型配置，使开发者加入类型和少量配置，即可支持该类型解析
2. Engine 功能实现可拔插，只要底层实现了 Connection 和 Channel 的主要功能，即可接入引擎
3. 增加 mustbind 进行绑定字段的强制校验，shouldbind 通过 tag:"binding" 进行强制校验控制
4. 接入链路追踪 opentelemetry
