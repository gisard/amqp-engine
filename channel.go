package amqp

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
	"google.golang.org/protobuf/proto"
)

const (
	// deadPrefix is prefix of dead queue
	deadPrefix = "dead-"
)

// amqp channel, user should close manually when no longer used.
type channel struct {
	// amqp connection to channel reconnect.
	conn Connection
	// amqp091-go channel to perform low-level operations.
	amqpChan *amqp.Channel
	// Daemon channel to accept amqp091-go channel close error.
	closeChan chan *amqp.Error
	// publish ack channel.
	publishChan chan amqp.Confirmation
	// Ensure concurrent publish is thread safety.
	mu sync.Mutex
}

/*
CreateExchange create exchange with name and type.
If the exchange does not already exist, the server will create it.
If the exchange exists, the server verifies that it is of the provided type.
Dead is prefix of dead-exchange, therefore exchange name should not start with dead.
*/
func (c *channel) CreateExchange(exchange string, exchangeType ExchangeType) error {
	if _, ok := ExchangeTypeMap[exchangeType]; !ok {
		return ErrExchangeTypeIsAbnormal
	}
	if exchange == DefaultExchangeName {
		return nil
	}

	defer c.mu.Unlock()
	c.mu.Lock()

	var err error
	for i := 0; i < defaultRetryNum; i++ {
		err = c.amqpChan.ExchangeDeclare(exchange, string(exchangeType), true,
			false, false, false, nil)
		if err != nil {
			if err == amqp.ErrClosed {
				_ = c.Reconnect()
			}
			continue
		}
		return nil
	}
	return err
}

// CreateQueue creates a queue if it doesn't already exist.
// It will declare a dead-exchange and a dead-queue to accept nack and reject message.
// Dead is prefix of dead-queue, therefore queue name should not start with dead.
func (c *channel) CreateQueues(names ...string) error {
	if len(names) == 0 {
		return nil
	}
	for _, name := range names {
		err := c.createQueue(name)
		if err != nil {
			return err
		}
	}
	return nil
}

// createQueue creates a queue if it doesn't already exist.
// It will declare a dead-exchange and a dead-queue to accept nack and reject message.
func (c *channel) createQueue(name string) error {
	if name == "" {
		return ErrQueueNameIsEmpty
	}

	defer c.mu.Unlock()
	c.mu.Lock()

	var err error
	for i := 0; i < defaultRetryNum; i++ {
		if strings.HasPrefix(name, deadPrefix) {
			// Create queue with dead exchange and dead queue.
			_, err = c.amqpChan.QueueDeclare(name, true, false,
				false, false, nil)
			if err != nil {
				if errors.Is(err, amqp.ErrClosed) {
					_ = c.Reconnect()
				}
				continue
			}
		} else {
			deadQueueName := fmt.Sprintf(deadLetterQueueFmt, name)
			// Add dead letter exchange.
			err = c.amqpChan.ExchangeDeclare(defaultDeadLetterExchange, string(Direct), true,
				false, false, false, nil)
			if err != nil {
				if errors.Is(err, amqp.ErrClosed) {
					_ = c.Reconnect()
				}
				continue
			}
			// Add dead letter queue.
			_, err = c.amqpChan.QueueDeclare(deadQueueName, true, false,
				false, false, nil)
			if err != nil {
				if errors.Is(err, amqp.ErrClosed) {
					_ = c.Reconnect()
				}
				continue
			}
			// Bind dead-queue to dead-exchange.
			err = c.amqpChan.QueueBind(deadQueueName, deadQueueName, defaultDeadLetterExchange,
				false, nil)
			if err != nil {
				if err == amqp.ErrClosed {
					_ = c.Reconnect()
				}
				continue
			}
			// Create queue with dead exchange and dead queue.
			_, err = c.amqpChan.QueueDeclare(name, true, false,
				false, false, amqp.Table{
					"x-dead-letter-exchange":    defaultDeadLetterExchange,
					"x-dead-letter-routing-key": deadQueueName,
				})
			if err != nil {
				if errors.Is(err, amqp.ErrClosed) {
					_ = c.Reconnect()
				}
				continue
			}
		}
		return nil
	}
	return err
}

// Bind binds queues to exchange.
func (c *channel) Bind(exchange string, queues ...string) error {
	if len(queues) == 0 {
		return nil
	}
	if exchange == DefaultExchangeName {
		return nil
	}

	for _, queue := range queues {
		err := c.bind(exchange, queue)
		if err != nil {
			return err
		}
	}
	return nil
}

// Bind binds queue to exchange.
func (c *channel) bind(exchange string, queue string) error {
	if queue == "" {
		return ErrQueueNameIsEmpty
	}
	if exchange == DefaultExchangeName {
		return nil
	}

	defer c.mu.Unlock()
	c.mu.Lock()

	var err error
	for i := 0; i < defaultRetryNum; i++ {
		err = c.amqpChan.QueueBind(queue, queue, exchange, false, nil)
		if err != nil {
			if err == amqp.ErrClosed {
				_ = c.Reconnect()
			}
			continue
		}
		return nil
	}
	return err
}

/*
DeleteExchange removes the named exchange from the server.
If there are queues binding with exchange will unbind.
If the exchange is not exist return nil.
*/
func (c *channel) DeleteExchange(name string) error {
	if name == DefaultExchangeName {
		return ErrDefaultExchangeNotAllowDelete
	}

	defer c.mu.Unlock()
	c.mu.Lock()

	var err error
	for i := 0; i < defaultRetryNum; i++ {
		err = c.amqpChan.ExchangeDelete(name, false, false)
		if err != nil {
			if err == amqp.ErrClosed {
				_ = c.Reconnect()
			}
			continue
		}
		return nil
	}
	return err
}

/*
DeleteQueues the queues will be deleted regardless there are any consumers on the queue.
Unbind with exchanges if the queues are bound.
*/
func (c *channel) DeleteQueues(names ...string) error {
	if len(names) == 0 {
		return nil
	}
	for _, name := range names {
		err := c.deleteQueue(name)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *channel) deleteQueue(name string) error {
	if name == "" {
		return ErrQueueNameIsEmpty
	}

	defer c.mu.Unlock()
	c.mu.Lock()

	var err error
	for i := 0; i < defaultRetryNum; i++ {
		_, err = c.amqpChan.QueueDelete(name, false, false, false)
		if err != nil {
			if err == amqp.ErrClosed {
				_ = c.Reconnect()
			}
			continue
		}
		return nil
	}
	return err
}

// Unbind removes binding between an exchange and queues with keys which is same to queue name.
func (c *channel) Unbind(exchange string, queues ...string) error {
	if len(queues) == 0 {
		return nil
	}
	if exchange == DefaultExchangeName {
		return ErrDefaultExchangCannotUnbind
	}

	for _, queue := range queues {
		err := c.unbind(exchange, queue)
		if err != nil {
			return err
		}
	}
	return nil
}

// unbind removes a binding between an exchange and queue with key which is same to queue name.
func (c *channel) unbind(exchange string, queue string) error {
	if queue == "" {
		return ErrQueueNameIsEmpty
	}
	if exchange == DefaultExchangeName {
		return ErrDefaultExchangCannotUnbind
	}

	defer c.mu.Unlock()
	c.mu.Lock()

	var err error
	for i := 0; i < defaultRetryNum; i++ {
		err = c.amqpChan.QueueUnbind(queue, queue, exchange, nil)
		if err != nil {
			if err == amqp.ErrClosed {
				_ = c.Reconnect()
			}
			continue
		}
		return nil
	}
	return err
}

/*
Publish publishing with default context-type, persistent, zero priority.
User must create exchange and queue, then bind them.
If exchange is fanout, queue name should be empty.
*/
func (c *channel) Publish(exchange string, queue string, content []byte) (Ack, error) {
	return c.publish(exchange, queue, content)
}

/*
publish publishing with unmandatory and mediate.
Publishings can be undeliverable when the mandatory flag is true and no queue is
bound that matches the routing key, or when the immediate flag is true and no
consumer on the matched queue is ready to accept the delivery.
*/
func (c *channel) publish(exchange string, queue string, content []byte) (Ack, error) {
	defer c.mu.Unlock()
	c.mu.Lock()

	var err error
	for i := 0; i < defaultRetryNum; i++ {
		// Goroutine will exit until message deliver successful.
		err = c.amqpChan.Publish(exchange, queue, false, false,
			amqp.Publishing{
				ContentType:  defaultContentType,
				DeliveryMode: uint8(persistent),
				Priority:     uint8(priorityZero),
				Body:         content,
			})
		if err != nil {
			if err == amqp.ErrClosed {
				_ = c.Reconnect()
			}
			continue
		}
		confirm := <-c.publishChan
		if confirm.Ack {
			return Ack(confirm.Ack), nil
		}
	}
	return UnAcknowledge, err
}

// PublishJson publish with json format and return publish result.
// If exchange is fanout, queue name should be empty.
func (c *channel) PublishJson(exchange string, queue string, content interface{}) (Ack, error) {
	jsonContent, err := json.Marshal(content)
	if err != nil {
		return UnAcknowledge, err
	}
	return c.Publish(exchange, queue, jsonContent)
}

// PublishProto publish with proto format and return publish result.
// If exchange is fanout, queue name should be empty.
func (c *channel) PublishProto(exchange string, queue string, content proto.Message) (Ack, error) {
	protoContent, err := proto.Marshal(content)
	if err != nil {
		return UnAcknowledge, err
	}
	return c.Publish(exchange, queue, protoContent)
}

// PublishDefault will create queue, and publish to it by default exchange.
func (c *channel) PublishDefault(queue string, content []byte) (Ack, error) {
	err := c.CreateQueues(queue)
	if err != nil {
		return UnAcknowledge, err
	}
	return c.publish(DefaultExchangeName, queue, content)
}

// PublishDefaultJson will create default queue,
// and publish to it by default exchange with json format.
func (c *channel) PublishDefaultJson(queue string, content interface{}) (Ack, error) {
	jsonContent, err := json.Marshal(content)
	if err != nil {
		return UnAcknowledge, err
	}
	return c.PublishDefault(queue, jsonContent)
}

// PublishDefaultProto will create default queue,
// and publish to it by default exchange with proto format.
func (c *channel) PublishDefaultProto(queue string, content proto.Message) (Ack, error) {
	protoContent, err := proto.Marshal(content)
	if err != nil {
		return UnAcknowledge, err
	}
	return c.PublishDefault(queue, protoContent)
}

// Subscribe get a channel of deliveries from specific queue.
func (c *channel) Subscribe(queue string) (<-chan amqp.Delivery, error) {
	if queue == "" {
		return nil, ErrQueueNameIsEmpty
	}

	var (
		deliveryChan <-chan amqp.Delivery
		err          error
	)
	for i := 0; i < defaultRetryNum; i++ {
		deliveryChan, err = c.amqpChan.Consume(queue,
			"",
			false,
			// When exclusive is false, the server will fairly distribute
			// deliveries across multiple consumers.
			false,
			// The noLocal flag is not supported by RabbitMQ.
			false,
			// When noWait is true, do not wait for the server to confirm the request and
			// immediately begin deliveries.
			false,
			nil,
		)
		if err != nil {
			if err == amqp.ErrClosed {
				_ = c.Reconnect()
			}
			continue
		}
		return deliveryChan, nil
	}
	return nil, err
}

// Return the underlying amqp channel if current methods do not meet your needs.
func (c *channel) GetAMQPChannel() *amqp.Channel {
	return c.amqpChan
}

func (c *channel) getCloseChan() chan *amqp.Error {
	return c.closeChan
}

func (c *channel) getPublishChan() chan amqp.Confirmation {
	return c.publishChan
}

// channel Reconnect when closed.
func (c *channel) Reconnect() error {
	ch, err := c.conn.NewChannel()
	if err != nil {
		return err
	}
	c.amqpChan = ch.GetAMQPChannel()
	c.closeChan = ch.getCloseChan()
	c.publishChan = ch.getPublishChan()
	return nil
}

// NotifyClose registers a listener for closed error events.
func (c *channel) NotifyClose() <-chan *amqp.Error {
	return c.closeChan
}

// IsClosed return amqp channel is closed.
func (c *channel) IsClosed() bool {
	return c.amqpChan.IsClosed()
}

// Close amqp channel.
func (c *channel) Close() error {
	if c.amqpChan != nil && !c.amqpChan.IsClosed() {
		return c.amqpChan.Close()
	}
	return nil
}
