package subpub

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestSubscribeAndPublish(t *testing.T) {
	sp := NewSubPub()

	subject := "test-subject"
	testMsg := "test-message"

	msgCh := make(chan interface{}, 1)

	sub, err := sp.Subscribe(subject, func(msg interface{}) {
		msgCh <- msg
	})

	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	err = sp.Publish(subject, testMsg)
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	select {
	case receivedMsg := <-msgCh:
		if receivedMsg != testMsg {
			t.Errorf("Expected message %v, got %v", testMsg, receivedMsg)
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for message")
	}

	sub.Unsubscribe()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	sp.Close(ctx)
}

func TestUnsubscribe(t *testing.T) {
	sp := NewSubPub()

	subject := "test-subject"
	testMsg := "test-message"

	count := 0
	var mu sync.Mutex

	sub, err := sp.Subscribe(subject, func(msg interface{}) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	err = sp.Publish(subject, testMsg)
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	sub.Unsubscribe()

	err = sp.Publish(subject, testMsg)
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if count != 1 {
		t.Errorf("Expected 1 message, got %d", count)
	}
	mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	sp.Close(ctx)
}

func TestSlowSubscriber(t *testing.T) {
	sp := NewSubPub()

	subject := "test-subject"
	testMsg := "test-message"

	fastCh := make(chan interface{}, 1)

	slowCh := make(chan interface{}, 1)

	_, err := sp.Subscribe(subject, func(msg interface{}) {
		fastCh <- msg
	})

	if err != nil {
		t.Fatalf("Failed to subscribe fast handler: %v", err)
	}

	_, err = sp.Subscribe(subject, func(msg interface{}) {
		time.Sleep(500 * time.Millisecond)
		slowCh <- msg
	})

	if err != nil {
		t.Fatalf("Failed to subscribe slow handler: %v", err)
	}

	start := time.Now()
	err = sp.Publish(subject, testMsg)
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	select {
	case receivedMsg := <-fastCh:
		if receivedMsg != testMsg {
			t.Errorf("Fast subscriber: Expected message %v, got %v", testMsg, receivedMsg)
		}
		elapsed := time.Since(start)
		if elapsed > 100*time.Millisecond {
			t.Errorf("Fast subscriber was delayed: %v", elapsed)
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for fast subscriber")
	}

	select {
	case receivedMsg := <-slowCh:
		if receivedMsg != testMsg {
			t.Errorf("Slow subscriber: Expected message %v, got %v", testMsg, receivedMsg)
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for slow subscriber")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	sp.Close(ctx)
}

func TestClose(t *testing.T) {
	t.Run("Context canceled", func(t *testing.T) {
		sp := NewSubPub()
		subject := "test-subject"

		_, err := sp.Subscribe(subject, func(msg interface{}) {
			time.Sleep(500 * time.Millisecond)
		})

		if err != nil {
			t.Fatalf("Failed to subscribe: %v", err)
		}

		err = sp.Publish(subject, "test-message")
		if err != nil {
			t.Fatalf("Failed to publish: %v", err)
		}

		time.Sleep(50 * time.Millisecond)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		start := time.Now()

		err = sp.Close(ctx)
		elapsed := time.Since(start)

		if elapsed > 100*time.Millisecond {
			t.Errorf("Close took too long with canceled context: %v", elapsed)
		}

		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context.Canceled error, got: %v", err)
		}
	})

	t.Run("Handlers continue after Close", func(t *testing.T) {
		sp := NewSubPub()
		subject := "test-subject"
		handlerDone := make(chan struct{})
		handlerCalled := make(chan struct{})
		var once sync.Once
		var onceDone sync.Once

		_, err := sp.Subscribe(subject, func(msg interface{}) {
			once.Do(func() {
				close(handlerCalled)
			})
			time.Sleep(300 * time.Millisecond)
			onceDone.Do(func() {
				close(handlerDone)
			})
		})

		if err != nil {
			t.Fatalf("Failed to subscribe: %v", err)
		}

		err = sp.Publish(subject, "test-message")
		if err != nil {
			t.Fatalf("Failed to publish: %v", err)
		}

		select {
		case <-handlerCalled:
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Handler was not called")
		}

		ctx := context.Background()

		start := time.Now()

		err = sp.Close(ctx)
		if err != nil {
			t.Errorf("Close returned error: %v", err)
		}

		elapsed := time.Since(start)
		if elapsed > 200*time.Millisecond {
			t.Errorf("Close took too long: %v", elapsed)
		}

		select {
		case <-handlerDone:
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Handler did not complete after Close")
		}

		err = sp.Publish(subject, "message-after-close")
		if err != nil {
			t.Errorf("Failed to publish after close: %v", err)
		}
	})
}

func TestMessageOrder(t *testing.T) {
	sp := NewSubPub()

	subject := "test-subject"

	msgCh := make(chan interface{}, 10)

	sub, err := sp.Subscribe(subject, func(msg interface{}) {
		msgCh <- msg
	})

	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	messages := []string{"first", "second", "third", "fourth", "fifth"}
	for _, msg := range messages {
		err = sp.Publish(subject, msg)
		if err != nil {
			t.Fatalf("Failed to publish: %v", err)
		}
	}

	for i, expected := range messages {
		select {
		case received := <-msgCh:
			if received != expected {
				t.Errorf("Message %d: expected %q, got %q", i, expected, received)
			}
		case <-time.After(time.Second):
			t.Fatalf("Timeout waiting for message %d", i)
		}
	}

	sub.Unsubscribe()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	sp.Close(ctx)
}

func TestPublishAfterClose(t *testing.T) {
	sp := NewSubPub()

	subject := "test-subject"

	msgCh := make(chan interface{}, 10)

	sub, err := sp.Subscribe(subject, func(msg interface{}) {
		msgCh <- msg
	})

	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	go func() {
		err := sp.Close(ctx)
		if err != nil {
			t.Errorf("Close returned error: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)

	testMsg := "message-after-close"
	err = sp.Publish(subject, testMsg)
	if err != nil {
		t.Fatalf("Failed to publish after close: %v", err)
	}

	select {
	case received := <-msgCh:
		if received != testMsg {
			t.Errorf("Expected message %q, got %q", testMsg, received)
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for message after close")
	}

	_, err = sp.Subscribe(subject, func(msg interface{}) {})
	if err == nil {
		t.Error("Expected error when subscribing after close, got nil")
	}

	sub.Unsubscribe()
}
