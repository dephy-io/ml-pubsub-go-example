package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/btcsuite/btcutil/bech32"
	"github.com/nbd-wtf/go-nostr"
)

// Function to decode npub keys to hex
func decodeNpub(npub string) (string, error) {
	// Decode the Bech32-encoded npub key
	_, data, err := bech32.Decode(npub)
	if err != nil {
		return "", fmt.Errorf("failed to decode npub key: %v", err)
	}

	// Convert the decoded data to a byte slice
	decodedBytes, err := bech32.ConvertBits(data, 5, 8, false)
	if err != nil {
		return "", fmt.Errorf("failed to convert bits: %v", err)
	}

	// Encode the byte slice to a hex string
	return hex.EncodeToString(decodedBytes), nil
}

func shuffle(array []string) []string {
	for i := len(array) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		array[i], array[j] = array[j], array[i]
	}
	return array
}

func main() {
	if len(os.Args) < 4 {
		fmt.Println("usage: go run publisher/main.go <relay endpoint> <public key array json file> <interval>")
		os.Exit(1)
	}

	relayEndpoint := os.Args[1]
	publicKeysPath := os.Args[2]
	interval, err := strconv.Atoi(os.Args[3])
	if err != nil {
		fmt.Printf("invalid interval: %v\n", err)
		os.Exit(1)
	}

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

	// Load public keys from JSON file
	fileContent, err := os.ReadFile(publicKeysPath)
	if err != nil {
		fmt.Printf("failed to read public keys file: %v\n", err)
		os.Exit(1)
	}
	var subscriberPublicKeys []string
	if err := json.Unmarshal(fileContent, &subscriberPublicKeys); err != nil {
		fmt.Printf("failed to parse public keys: %v\n", err)
		os.Exit(1)
	}

	// Decode npub keys to hex
	var decodedPublicKeys []string
	for _, npub := range subscriberPublicKeys {
		hexKey, err := decodeNpub(npub)
		if err != nil {
			fmt.Printf("failed to decode npub key %s: %v\n", npub, err)
			continue
		}
		decodedPublicKeys = append(decodedPublicKeys, hexKey)
	}

    fmt.Println("decodedPublicKeys:", decodedPublicKeys)

	// Generate keys
	sk := nostr.GeneratePrivateKey()
	pk, err := nostr.GetPublicKey(sk)
	if err != nil {
		fmt.Printf("failed to generate public key: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("publisher public key: %s\n", pk)

	// Subscribe to pong messages
	go func() {
		since := nostr.Now()
		filters := nostr.Filters{{
			Kinds: []int{1573},
			Since: &since,
			Tags:  nostr.TagMap{"s": []string{"pingpong"}, "p": []string{pk}},
		}}
		sub, err := relay.Subscribe(ctx, filters)
		if err != nil {
			fmt.Printf("failed to subscribe: %v\n", err)
			return
		}
		defer sub.Close()

		for event := range sub.Events {
			if event.Content == "pong" {
				fmt.Printf("got pong from %s\n", event.PubKey)
			}
		}
	}()

	// Main loop to send pings
	for {
		shuffledPublicKeys := shuffle(decodedPublicKeys)
		fmt.Printf("shuffled public keys: %v\n", shuffledPublicKeys)

		for _, pubkey := range shuffledPublicKeys {
			event := nostr.Event{
				PubKey:    pk,
				CreatedAt: nostr.Now(),
				Kind:      1573,
				Tags:      nostr.Tags{{"s", "pingpong"}, {"p", pubkey}},
				Content:   "ping",
			}

			if err := event.Sign(sk); err != nil {
				fmt.Printf("failed to sign event: %v\n", err)
				continue
			}

			if err := relay.Publish(ctx, event); err != nil {
				fmt.Printf("failed to publish event: %v\n", err)
				continue
			}
			fmt.Printf("sent ping to %s\n", pubkey)
			time.Sleep(time.Duration(interval) * time.Second)
		}
	}
}