package main

import (
	"context"
	"fmt"
	"os"

	"github.com/nbd-wtf/go-nostr"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println(os.Args)
		fmt.Println("usage: go run subscriber/main.go <relay endpoint> <hex secret key>")
		os.Exit(1)
	}

	relayEndpoint := os.Args[1]
	sk := os.Args[2]

	// Create a context
	ctx := context.Background()

	// Connect to the relay
	relay, err := nostr.RelayConnect(ctx, relayEndpoint)
	if err != nil {
		fmt.Printf("failed to connect to relay: %v\n", err)
		os.Exit(1)
	}
	defer relay.Close()
	fmt.Printf("connected to relay %s\n", relayEndpoint)

	pk, err := nostr.GetPublicKey(sk)
	if err != nil {
		fmt.Printf("failed to generate public key: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("subscriber public key: %s\n", pk)

	// Subscribe to ping messages and respond with pong
	since := nostr.Now()
	filters := nostr.Filters{{
		Kinds: []int{1573},
		Since: &since,
		Tags:  nostr.TagMap{"s": []string{"pingpong"}, "p": []string{pk}},
	}}
	sub, err := relay.Subscribe(ctx, filters)
	if err != nil {
		fmt.Printf("failed to subscribe: %v\n", err)
		os.Exit(1)
	}
	defer sub.Close()

	for event := range sub.Events {
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
				continue
			}
			fmt.Printf("sent pong to %s\n", event.PubKey)
		}
	}
}
