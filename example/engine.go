package main

import (
	"fmt"

	"github.com/guoshaodong/amqp-engine/engine"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

type consumeStruct struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func main() {
	var (
		amqpURL = "amqp://username:password@localhost:5672/"

		defaultQueueName = "ctx-queue"
		jsonQueueName    = "json-queue"
		protoQueueName   = "proto-queue"

		publishDefaultQueueName      = "publish-default-queue"
		publishDefaultJsonQueueName  = "publish-default-json-queue"
		publishDefaultProtoQueueName = "publish-default-proto-queue"

		ctxKey   = "key:1"
		ctxValue = "value:2"
	)

	e := engine.New(amqpURL)

	e.Use(func(ctx *engine.Context) {
		ctx.Set(ctxKey, ctxValue)
	})

	e.Handle(defaultQueueName, false, func(ctx *engine.Context) error {
		fmt.Println(ctx.Value(ctxKey))
		content := ctx.GetContent()
		logrus.Infof("获取 %s 内容为：%s", defaultQueueName, content)
		return nil
	})

	defaultGroup := e.Group("default", func(ctx *engine.Context) {
		logrus.Info("开始处理 default")
		ctx.Next()
		logrus.Info("结束处理 default")
	})
	defaultGroup.Handle(publishDefaultQueueName, false, func(ctx *engine.Context) error {
		content := ctx.GetContent()
		logrus.Infof("获取 %s 内容为：%s", publishDefaultQueueName, content)
		return nil
	})

	jsonGroup := e.Group("json", func(ctx *engine.Context) {
		logrus.Info("开始处理 json")
		ctx.Next()
		logrus.Info("结束处理 json")
	})
	jsonGroup.Handle(jsonQueueName, false, func(ctx *engine.Context) error {
		var s1 consumeStruct
		err := ctx.ShouldBindJson(&s1)
		if err != nil {
			return err
		}
		logrus.Infof("获取 %s 内容为：%v", jsonQueueName, s1)
		return nil
	})
	jsonGroup.Handle(publishDefaultJsonQueueName, false, func(ctx *engine.Context) error {
		var s1 consumeStruct
		err := ctx.ShouldBindJson(&s1)
		if err != nil {
			return err
		}
		logrus.Infof("获取 %s 内容为：%v", jsonQueueName, s1)
		return nil
	})

	protoGroup := e.Group("proto", func(ctx *engine.Context) {
		logrus.Info("开始处理 proto")
		ctx.Next()
		logrus.Info("结束处理 proto")
	})
	protoGroup.Handle(protoQueueName, false, func(ctx *engine.Context) error {
		var p1 proto.Message
		err := ctx.ShouldBindProto(&p1)
		if err != nil {
			return err
		}
		logrus.Infof("获取 %s 内容为：%v", publishDefaultProtoQueueName, p1)
		return nil
	})
	protoGroup.Handle(publishDefaultProtoQueueName, false, func(ctx *engine.Context) error {
		var p1 proto.Message
		err := ctx.ShouldBindProto(&p1)
		if err != nil {
			return err
		}
		logrus.Infof("获取 %s 内容为：%v", publishDefaultProtoQueueName, p1)
		return nil
	})

	e.Run()
}
