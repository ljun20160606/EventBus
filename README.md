eventbus
======

Package eventbus is the little and lightweight eventbus with async compatibility for GoLang.

#### Example
```go
func calculator(a int, b int) {
	fmt.Printf("%d\n", a + b)
}

func main() {
	bus := eventbus.New();
	bus.Subscribe("main:calculator", calculator);
	bus.Publish("main:calculator", 20, 40);
	bus.Unsubscribe("main:calculator", calculator);
}
```

#### Implemented methods
* **New()**
* **Subscribe()**
* **SubscribeOnce()**
* **HasCallback()**
* **Unsubscribe()**
* **Publish()**
* **WaitAsync()**

#### New()
New returns new eventbus with empty handlers.
```go
bus := eventbus.New();
```

#### Subscribe(topic string, fn interface{}) error
Subscribe to a topic. Returns error if `fn` is not a function.
```go
func Handler() { ... }
...
bus.Subscribe("topic:handler", Handler)
```

#### SubscribeOnce(topic string, fn interface{}) error
Subscribe to a topic once. Handler will be removed after executing. Returns error if `fn` is not a function.
```go
func HelloWorld() { ... }
...
bus.Subscribe("topic:handler", HelloWorld, eventbus.WithOnce())
```

#### Unsubscribe(topic string, fn interface{}) error
Remove callback defined for a topic. Returns error if there are no callbacks subscribed to the topic.
```go
bus.Unsubscribe("topic:handler", HelloWord);
```

#### HasCallback(topic string) bool
Returns true if exists any callback subscribed to the topic.

#### Publish(topic string, args ...interface{})
Publish executes callback defined for a topic. Any additional argument will be transferred to the callback.
```go
func Handler(str string) { ... }
...
bus.Subscribe("topic:handler", Handler)
...
bus.Publish("topic:handler", "Hello, World!");
```

#### SubscribeAsync(topic string, fn interface{})
Subscribe to a topic with an asynchronous callback. Returns error if `fn` is not a function.
```go
func slowCalculator(a, b int) {
	time.Sleep(3 * time.Second)
	fmt.Printf("%d\n", a + b)
}

bus := eventbus.New()
bus.Subscribe("main:slow_calculator", slowCalculator, eventbus.WithAsync(), eventbus.WithTransactional())

bus.Publish("main:slow_calculator", 20, 60)

fmt.Println("start: do some stuff while waiting for a result")
fmt.Println("end: do some stuff while waiting for a result")

bus.WaitAsync() // wait for all async callbacks to complete

fmt.Println("do some stuff after waiting for result")
```
Transactional determines whether subsequent callbacks for a topic are run serially (true) or concurrently(false)

#### SubscribeOnceAsync(topic string, args ...interface{})
SubscribeOnceAsync works like SubscribeOnce except the callback to executed asynchronously

####  WaitAsync()
WaitAsync waits for all async callbacks to complete.

#### Support
If you do have a contribution for the package feel free to put up a Pull Request or open Issue.
