package amqp

import (
	"errors"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"google.golang.org/protobuf/proto"
)

var (
	ErrConnectionRetryFail = errors.New("amqp connection retry failed")

	ErrChannelOpenFail = errors.New("amqp channel is not open")

	ErrExchangeTypeIsAbnormal = errors.New("exchange type is abnormal")

	ErrQueueNameIsEmpty = errors.New("queue name is empty")

	ErrQueueIsAbnormal = errors.New("queue is abnormal")

	ErrDefaultExchangeNotAllowDelete = errors.New("default exchange not allow to delete")

	ErrDefaultExchangCannotUnbind = errors.New("default exchange can't unbind")
)

const (
	defaultHeartTime = 3 * time.Second

	defaultRetryNum = 5

	defaultOnce = 1
)

const (
	DefaultExchangeName = ""

	defaultContentType = "text/plain" // nolint
)

const (
	defaultDeadLetterExchange = "dead-exchange-default"

	deadLetterQueueFmt = "dead-%s"
)

type ExchangeType string

const (
	Direct  ExchangeType = "direct"  // Direct exchange
	Fanout  ExchangeType = "fanout"  // Fanout exchange
	Topic   ExchangeType = "topic"   // Topic exchange
	Headers ExchangeType = "headers" // Headers exchange
)

var (
	ExchangeTypeMap = map[ExchangeType]string{
		Direct:  "direct",
		Fanout:  "fanout",
		Topic:   "topic",
		Headers: "headers",
	}
)

/*
DeliveryMode. Transient means higher throughput but messages will not be
restored on broker restart.  The delivery mode of publishings is unrelated
to the durability of the queues they reside on.  Transient messages will
not be restored to durable queues, persistent messages will be restored to
durable queues and lost on non-durable queues during server restart.
*/
type deliveryMode uint8

// nolint
const (
	transient deliveryMode = iota + 1 // Transient means higher throughput
	persistent
)

// priority 0 to 9
type priority uint8

// nolint
const (
	priorityZero priority = iota
)

// Ack is used to Acknowledge consume message.
type Ack bool

const (
	Acknowledge   Ack = true
	UnAcknowledge Ack = false
)

type Format string

const (
	JsonFormat  = "json"
	ProtoFormat = "proto"
)

type Connection interface {
	Reconnect() error
	NotifyClose() chan *amqp.Error
	IsClosed() bool
	Close() error
	NewChannel() (Channel, error)
}

type Channel interface {
	CreateExchange(name string, exchangeType ExchangeType) error
	CreateQueues(names ...string) error
	Bind(exchange string, queue ...string) error

	DeleteExchange(name string) error
	DeleteQueues(name ...string) error
	Unbind(exchange string, queues ...string) error

	Publish(exchange string, queue string, content []byte) (Ack, error)
	PublishJson(exchange string, queue string, content interface{}) (Ack, error)
	PublishProto(exchange string, queue string, content proto.Message) (Ack, error)

	// Publish to queue with default exchange.
	PublishDefault(queue string, content []byte) (Ack, error)
	PublishDefaultJson(queue string, content interface{}) (Ack, error)
	PublishDefaultProto(queue string, content proto.Message) (Ack, error)

	Subscribe(queue string) (<-chan amqp.Delivery, error)

	GetAMQPChannel() *amqp.Channel
	getCloseChan() chan *amqp.Error
	getPublishChan() chan amqp.Confirmation

	Reconnect() error
	NotifyClose() <-chan *amqp.Error
	IsClosed() bool
	Close() error
}
