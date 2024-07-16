package main

import (
	"github.com/gisard/amqp-engine"
	"google.golang.org/protobuf/types/known/emptypb"
)

type testStruct struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

// nolint
var (
	amqpURL = "amqp://username:password@127.0.0.1:5672/"

	directExchange = "direct-exchange"
	fanoutExchange = "fanout-exchange"

	directQueue  = "direct-queue"
	fanoutQueue1 = "fanout-queue-1"
	fanoutQueue2 = "fanout-queue-2"
	fanoutQueue3 = "fanout-queue-3"

	content     = []byte("Welcome to Asynchronous Messaging Engine !")
	jsonContent = testStruct{
		Name: "测试json",
		Age:  18,
	}
	protoContent = emptypb.Empty{}

	defaultQueueName      = "default-queue"
	defaultJsonQueueName  = "default-json-queue"
	defaultProtoQueueName = "default-proto-queue"
)

// nolint
func main() {
	conn, err := amqp.New(amqpURL)
	if err != nil {
		panic(err)
	}

	channel, err := conn.NewChannel()
	if err != nil {
		panic(err)
	}

	// create direct exchange
	err = channel.CreateExchange(directExchange, amqp.Direct)
	if err != nil {
		panic(err)
	}
	// create fanout exchange
	err = channel.CreateExchange(fanoutExchange, amqp.Fanout)
	if err != nil {
		panic(err)
	}

	// create queues
	err = channel.CreateQueues(directQueue, fanoutQueue1, fanoutQueue2, fanoutQueue3)
	if err != nil {
		panic(err)
	}

	// Bind
	err = channel.Bind(directExchange, directQueue)
	if err != nil {
		panic(err)
	}
	err = channel.Bind(fanoutExchange, fanoutQueue1, fanoutQueue2, fanoutQueue3)
	if err != nil {
		panic(err)
	}

	// Publish
	_, err = channel.Publish(directExchange, directQueue, content)
	if err != nil {
		panic(err)
	}
	_, err = channel.PublishJson(fanoutExchange, "", jsonContent)
	if err != nil {
		panic(err)
	}
	_, err = channel.PublishProto(fanoutExchange, "", &protoContent)
	if err != nil {
		panic(err)
	}

	// PublishDefault
	_, err = channel.PublishDefault(defaultQueueName, content)
	if err != nil {
		panic(err)
	}
	_, err = channel.PublishDefaultJson(defaultJsonQueueName, jsonContent)
	if err != nil {
		panic(err)
	}
	_, err = channel.PublishDefaultProto(defaultProtoQueueName, &protoContent)
	if err != nil {
		panic(err)
	}

	// unbind
	err = channel.Unbind(directExchange, directQueue)
	if err != nil {
		panic(err)
	}
	// delete queue
	err = channel.DeleteQueues(directQueue)
	if err != nil {
		panic(err)
	}
	// delete exchange
	err = channel.DeleteExchange(fanoutExchange)
	if err != nil {
		panic(err)
	}
}
