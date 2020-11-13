package rabbitmq

import (
	"fmt"
	"log"

	"github.com/streadway/amqp"
	"github.com/toolkits/pkg/logger"
)

var (
	conn *amqp.Connection // 全局存 mq 连接句柄
	exit = make(chan bool)
)

func Init(url string) {
	var err error
	conn, err = amqp.Dial(url)
	if err != nil {
		log.Fatal(err)
	}
}

// ping 测试rabbitmq连接是否正常
func ping() (err error) {
	if conn == nil {
		return fmt.Errorf("conn is nil")
	}

	ch, err := conn.Channel()
	if err != nil {
		logger.Error(err)
		close()
		return err
	}

	defer ch.Close()
	// 声明topic
	err = ch.ExchangeDeclare("ping.ping", "topic", false, true, false, true, nil)
	if err != nil {
		logger.Error(err)
		return err
	}

	msgContent := "ping.ping"
	err = ch.Publish("ping.ping", "ping.ping", false, false, amqp.Publishing{ // 测试推一条消息
		ContentType: "text/plain",
		Body:        []byte(msgContent),
	})
	if err != nil {
		logger.Error(err)
		return err
	}
	err = ch.ExchangeDelete("ping.ping", false, false) // 删除 topic
	if err != nil {
		logger.Error(err)
	}

	return err
}

func close() {
	if conn != nil {
		err := conn.Close()
		if err != nil {
			logger.Error(err)
		}
		conn = nil
	}
}

func Shutdown() {
	conn.Close()
	exit <- true
}
