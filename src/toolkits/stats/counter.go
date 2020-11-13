package stats

import "sync"

type CounterMetric struct {
	sync.RWMutex
	prefix  string
	metrics map[string]int // 统计一些，错误或者请求接口 的数量
}

var Counter *CounterMetric // 统计qps

func NewCounter(prefix string) *CounterMetric {
	return &CounterMetric{
		metrics: make(map[string]int),
		prefix:  prefix, // 设置前缀，每个对象唯一，取值时候，用于拼接key
	}
}

func (c *CounterMetric) Set(metric string, value int) {
	c.Lock()
	defer c.Unlock()
	if _, exists := c.metrics[metric]; exists {
		c.metrics[metric] += value
	} else {
		c.metrics[metric] = value
	}
}

func (c *CounterMetric) Dump() map[string]int { // 取值后，旧数据清理
	c.Lock()
	defer c.Unlock()
	metrics := make(map[string]int)
	for key, value := range c.metrics {
		newKey := c.prefix + "." + key
		metrics[newKey] = value
		c.metrics[key] = 0 // 取值后，置0
	}

	return metrics
}
