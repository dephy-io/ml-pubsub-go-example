package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

const (
	maxRetries     = 10
	initialDelay   = 1 * time.Second
	maxDelay       = 30 * time.Second
	reconnectDelay = 5 * time.Second
)

func connectWithRetry(ctx context.Context, relayEndpoint string) (*nostr.Relay, error) {
	var relay *nostr.Relay
	var err error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		relay, err = nostr.RelayConnect(ctx, relayEndpoint)
		if err == nil {
			return relay, nil
		}

		fmt.Printf("Connection attempt %d/%d failed: %v\n", attempt, maxRetries, err)

		if attempt < maxRetries {
			delay := initialDelay * time.Duration(attempt*attempt)
			if delay > maxDelay {
				delay = maxDelay
			}
			fmt.Printf("Retrying in %v...\n", delay)
			time.Sleep(delay)
		}
	}

	return nil, fmt.Errorf("failed to connect after %d attempts: %v", maxRetries, err)
}

func isConnectionError(err error) bool {
	return err != nil && (err.Error() == "connection closed" ||
		err.Error() == "write: broken pipe" ||
		err.Error() == "read: connection reset by peer")
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println(os.Args)
		fmt.Println("usage: go run subscriber/main.go <relay endpoint> <hex secret key>")
		os.Exit(1)
	}

	relayEndpoint := os.Args[1]
	sk := os.Args[2]
	ctx := context.Background()

	for {
		// Connect to the relay with retry
		relay, err := connectWithRetry(ctx, relayEndpoint)
		if err != nil {
			fmt.Printf("Failed to connect to relay: %v\n", err)
			time.Sleep(reconnectDelay)
			continue
		}

		pk, err := nostr.GetPublicKey(sk)
		if err != nil {
			fmt.Printf("failed to generate public key: %v\n", err)
			relay.Close()
			time.Sleep(reconnectDelay)
			continue
		}
		fmt.Printf("subscriber public key: %s\n", pk)

		// Subscribe to ping messages
		since := nostr.Now()
		filters := nostr.Filters{{
			Kinds: []int{1573},
			Since: &since,
			Tags:  nostr.TagMap{"s": []string{"pingpong"}, "p": []string{pk}},
		}}

		sub, err := relay.Subscribe(ctx, filters)
		if err != nil {
			fmt.Printf("failed to subscribe: %v\n", err)
			relay.Close()
			time.Sleep(reconnectDelay)
			continue
		}

		fmt.Println("Successfully subscribed, waiting for events...")

		// Process events
		for {
			select {
			case event, ok := <-sub.Events:
				if !ok {
					fmt.Println("Event channel closed, reconnecting...")
					sub.Close()
					relay.Close()
					time.Sleep(reconnectDelay)
					goto RECONNECT
				}

				if event.Content == "ping" {
					fmt.Printf("received ping from %s\n", event.PubKey)

					// Create and send pong response
					pongEvent := nostr.Event{
						PubKey:    pk,
						CreatedAt: nostr.Now(),
						Kind:      1573,
						Tags:      nostr.Tags{{"s", "pingpong"}, {"p", event.PubKey}},
						Content:   "pong",
					}

					if err := pongEvent.Sign(sk); err != nil {
						fmt.Printf("failed to sign pong event: %v\n", err)
						continue
					}

					if err := relay.Publish(ctx, pongEvent); err != nil {
						fmt.Printf("failed to publish pong event: %v\n", err)
						if isConnectionError(err) {
							sub.Close()
							relay.Close()
							time.Sleep(reconnectDelay)
							goto RECONNECT
						}
						continue
					}
					fmt.Printf("sent pong to %s\n", event.PubKey)
				}

			case <-ctx.Done():
				sub.Close()
				relay.Close()
				return
			}
		}

	RECONNECT:
		fmt.Println("Attempting to reconnect...")
	}
}
