package amqp

import (
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

// amqp connection.
type connection struct {
	// amqp url
	url string
	// amqp091-go connection
	amqpConn *amqp.Connection
	// Daemon channel to accept connection close error.
	closeChan chan *amqp.Error

	mu sync.Mutex
}

// New connect to amqp server and return a connection.
// url like amqp://username:password@127.0.0.1:5672/
func New(url string) (Connection, error) {
	conn := connection{
		url: url,
	}
	err := conn.connect()
	if err != nil {
		return nil, err
	}
	return &conn, nil
}

// connect lazy mode to create a connection.
// Create a daemon channel to listen connection close.
func (c *connection) connect() error {
	if c.amqpConn != nil && !c.amqpConn.IsClosed() {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.amqpConn != nil && !c.amqpConn.IsClosed() {
		return nil
	}

	conn, err := amqp.Dial(c.url)
	if err != nil {
		return err
	}

	c.amqpConn = conn
	c.closeChan = c.amqpConn.NotifyClose(make(chan *amqp.Error))

	return nil
}

// getConnection return c.amqpConn.
func (c *connection) getConnection() *amqp.Connection {
	return c.amqpConn
}

// Reconnect to amqp server.
func (c *connection) Reconnect() error {
	return c.reconnect(0)
}

// lazy mode to reconnect to amqp server
func (c *connection) reconnect(number int) error {
	if c.amqpConn != nil && !c.amqpConn.IsClosed() {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.amqpConn != nil && !c.amqpConn.IsClosed() {
		return nil
	}

	var (
		conn *amqp.Connection
		err  error
	)
	if number == 0 {
		for {
			// amqp091-go defaultHeartbeat is 10s.
			// amqp091-go defaultConnectionTimeout is 30s.
			conn, err = amqp.Dial(c.url)
			if err == nil {
				c.amqpConn = conn
				break
			}
			logrus.Info(ErrConnectionRetryFail)
			<-time.After(defaultHeartTime)
		}
	} else {
		for i := 0; i < number; i++ {
			conn, err = amqp.Dial(c.url)
			if err == nil {
				c.amqpConn = conn
				break
			}
			logrus.Info(ErrConnectionRetryFail)
			<-time.After(defaultHeartTime)
		}
	}
	if err == nil {
		c.closeChan = c.amqpConn.NotifyClose(make(chan *amqp.Error))
	}

	return err
}

// NotifyClose registers a listener for closed error events.
func (c *connection) NotifyClose() chan *amqp.Error {
	return c.closeChan
}

// IsClosed return amqp connect is closed.
func (c *connection) IsClosed() bool {
	return c.amqpConn.IsClosed()
}

// Close amqp connection.
func (c *connection) Close() error {
	if !c.amqpConn.IsClosed() {
		return c.amqpConn.Close()
	}
	return nil
}

// NewChannel instantiate a Channel.
func (c *connection) NewChannel() (Channel, error) {
	if c.IsClosed() {
		err := c.reconnect(defaultOnce)
		if err != nil {
			return nil, err
		}
	}
	ch, err := c.getConnection().Channel()
	if err != nil {
		return nil, err
	}
	// Confirm puts this channel into confirm mode.
	err = ch.Confirm(false)
	if err != nil {
		return nil, err
	}
	return &channel{
		conn:        c,
		amqpChan:    ch,
		closeChan:   ch.NotifyClose(make(chan *amqp.Error)),
		publishChan: ch.NotifyPublish(make(chan amqp.Confirmation)),
	}, nil
}
