package engine

import (
	"errors"

	"github.com/guoshaodong/amqp-engine"
	openamqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

type queue struct {
	// queue name
	name string
	// auto ack
	autoAck bool
	// amqp channel
	channel amqp.Channel

	// run chain
	handlers handlersChain
	// close err to main
	closeErr chan<- int8
	// the index of queues
	index int8
}

// queue start
func (q *queue) run() {
	defer func() {
		q.stop()
	}()
	// Create default queue with name will return err when queue is exist and ignore it.
	_ = q.channel.CreateQueues(q.name)

	deliveryChain, err := q.channel.Subscribe(q.name)
	for err != nil {
		if errors.Is(err, openamqp.ErrClosed) {
			err = q.channel.Reconnect()
		}
		if err != nil {
			logrus.Info(err)
			q.closeErr <- q.index
			return
		}
	}
	closeChan := q.channel.NotifyClose()
	logrus.Infof("Begin listening queue: %s\n", q.name)

	for {
		select {
		// closeErr has higher priority than delivery
		case closeErr := <-closeChan:
			// closeErr is nil, represent channel is closed normally.
			if closeErr == nil {
				return
			}
			// close exception will return the index of queue to main.
			logrus.Info(amqp.ErrChannelOpenFail)
			q.closeErr <- q.index
			return
		default:
			select {
			case closeErr := <-closeChan:
				if closeErr == nil {
					return
				}
				logrus.Info(amqp.ErrChannelOpenFail)
				q.closeErr <- q.index
				return
			case delivery := <-deliveryChain:
				// Acknowledger is used to determine delivery is Effective.
				if delivery.Acknowledger == nil {
					logrus.Info(amqp.ErrQueueIsAbnormal)
					q.closeErr <- q.index
					return
				}
				// allocate a context to handle delivery.
				ctx := newCtx(q, &delivery, q.handlers)
				go ctx.run()
			}
		}
	}
}

// queue stop
func (q *queue) stop() {
	if q.channel != nil && !q.channel.IsClosed() {
		q.channel.Close()
	}
}
