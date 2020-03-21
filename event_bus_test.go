package eventbus

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	ast := assert.New(t)

	bus := New()
	ast.NotNil(bus)
}

func TestHasCallback(t *testing.T) {
	ast := assert.New(t)

	bus := New()
	ast.Nil(bus.Subscribe("topic", func() {}))
	ast.False(bus.HasCallback("topic_topic"))
	ast.True(bus.HasCallback("topic"))
}

func TestSubscribe(t *testing.T) {
	ast := assert.New(t)

	bus := New()
	ast.Nil(bus.Subscribe("topic", func() {}))
	ast.NotNil(bus.Subscribe("topic", "String"))
}

func TestSubscribeOnce(t *testing.T) {
	ast := assert.New(t)

	bus := New()
	ast.Nil(bus.Subscribe("topic", func() {}, WithOnce()))
	ast.NotNil(bus.Subscribe("topic", "String", WithOnce()))
}

func TestSubscribeOnceAndManySubscribe(t *testing.T) {
	ast := assert.New(t)

	bus := New()
	event := "topic"
	flag := 0
	fn := func() { flag += 1 }
	ast.Nil(bus.Subscribe(event, fn, WithOnce()))
	ast.Nil(bus.Subscribe(event, fn))
	ast.Nil(bus.Subscribe(event, fn))

	ast.Nil(bus.Publish(event))
	ast.Equal(3, flag)

	ast.Nil(bus.Publish(event))
	ast.Equal(5, flag)
}

func TestUnsubscribe(t *testing.T) {
	ast := assert.New(t)

	bus := New()
	handler := func() {}
	ast.Nil(bus.Subscribe("topic", handler))
	ast.Nil(bus.Unsubscribe("topic", handler))
	ast.NotNil(bus.Unsubscribe("topic", handler))
}

func TestPublish(t *testing.T) {
	ast := assert.New(t)

	bus := New()
	ast.Nil(
		bus.Subscribe("topic", func(a int, b int) {
			ast.Equal(a, b)
		}),
	)
	ast.Nil(bus.Publish("topic", 10, 10))
}

func TestSubcribeOnceAsync(t *testing.T) {
	ast := assert.New(t)

	results := make([]int, 0)

	bus := New()
	ast.Nil(bus.Subscribe("topic", func(a int, out *[]int) {
		*out = append(*out, a)
	}, WithOnce(), WithAsync()))

	ast.Nil(bus.Publish("topic", 10, &results))
	ast.Nil(bus.Publish("topic", 10, &results))

	bus.WaitAsync()

	ast.Equal(len(results), 1)

	ast.False(bus.HasCallback("topic"))
}

func TestSubscribeAsyncTransactional(t *testing.T) {
	ast := assert.New(t)

	results := make([]int, 0)

	bus := New()
	ast.Nil(bus.Subscribe("topic", func(a int, out *[]int, dur string) {
		sleep, _ := time.ParseDuration(dur)
		time.Sleep(sleep)
		*out = append(*out, a)
	}, WithAsync(), WithTransactional()))

	ast.Nil(bus.Publish("topic", 1, &results, "1s"))
	ast.Nil(bus.Publish("topic", 2, &results, "0s"))

	bus.WaitAsync()

	ast.Equal(len(results), 2)

	ast.Equal(results[0], 1)
	ast.Equal(results[1], 2)
}

func TestSubscribeAsync(t *testing.T) {
	ast := assert.New(t)

	results := make(chan int)

	bus := New()
	ast.Nil(bus.Subscribe("topic", func(a int, out chan<- int) {
		out <- a
	}, WithAsync()))

	ast.Nil(bus.Publish("topic", 1, results))
	ast.Nil(bus.Publish("topic", 2, results))

	numResults := 0

	go func() {
		for range results {
			numResults++
		}
	}()

	bus.WaitAsync()

	time.Sleep(10 * time.Millisecond)

	ast.Equal(2, numResults)
}

func TestSubscribeOrder(t *testing.T) {
	ast := assert.New(t)

	var results []int

	bus := New()
	ast.Nil(
		bus.Subscribe("topic", func() {
			results = append(results, 2)
		}, WithOrder(2)),
	)

	ast.Nil(
		bus.Subscribe("topic", func() {
			results = append(results, 3)
		}, WithOrder(3)),
	)

	ast.Nil(
		bus.Subscribe("topic", func() {
			results = append(results, 1)
		}, WithOrder(1)),
	)

	ast.Nil(bus.Publish("topic"))

	ast.Equal([]int{3, 2, 1}, results)
}

func TestPublishReturnError(t *testing.T) {
	ast := assert.New(t)

	var flag int

	bus := New()
	ast.Nil(
		bus.Subscribe("topic", func() {
			flag += 1
		}, WithOrder(3)),
	)

	err := errors.New("topic err")
	ast.Nil(
		bus.Subscribe("topic", func() (interface{}, error) {
			return nil, err
		}, WithOrder(2)),
	)

	ast.Nil(
		bus.Subscribe("topic", func() {
			flag += 2
		}, WithOrder(1)),
	)

	// pub
	ast.Equal(bus.Publish("topic"), err)
	// interrupt by err
	ast.Equal(1, flag)
}
