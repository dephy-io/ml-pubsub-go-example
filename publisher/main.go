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

const (
	maxRetries     = 10
	initialDelay   = 1 * time.Second
	maxDelay       = 30 * time.Second
	reconnectDelay = 5 * time.Second
)

func decodeNpub(npub string) (string, error) {
	_, data, err := bech32.Decode(npub)
	if err != nil {
		return "", fmt.Errorf("failed to decode npub key: %v", err)
	}

	decodedBytes, err := bech32.ConvertBits(data, 5, 8, false)
	if err != nil {
		return "", fmt.Errorf("failed to convert bits: %v", err)
	}

	return hex.EncodeToString(decodedBytes), nil
}

func shuffle(array []string, r *rand.Rand) []string {
	for i := len(array) - 1; i > 0; i-- {
		j := r.Intn(i + 1)
		array[i], array[j] = array[j], array[i]
	}
	return array
}

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

	ctx := context.Background()

	// Load public keys from JSON file
	fileContent, err := os.ReadFile(publicKeysPath)
	if err != nil {
		fmt.Printf("failed to read public keys file: %v\n", err)
		return
	}

	var subscriberPublicKeys []string
	if err := json.Unmarshal(fileContent, &subscriberPublicKeys); err != nil {
		fmt.Printf("failed to parse public keys: %v\n", err)
		return
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

	// Generate keys
	sk := nostr.GeneratePrivateKey()
	pk, err := nostr.GetPublicKey(sk)
	if err != nil {
		fmt.Printf("failed to generate public key: %v\n", err)
		return
	}
	fmt.Printf("publisher public key: %s\n", pk)

	// Local random generator
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	for {
		// Connect to the relay with retry
		relay, err := connectWithRetry(ctx, relayEndpoint)
		if err != nil {
			fmt.Printf("Failed to connect to relay: %v\n", err)
			time.Sleep(reconnectDelay)
			continue
		}

		// Start pong listener in a goroutine
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

		// Main ping loop
	PING_LOOP:
		for {
			shuffledPublicKeys := shuffle(decodedPublicKeys, r)
			fmt.Printf("shuffled public keys: %v\n", shuffledPublicKeys)

			for _, pubkey := range shuffledPublicKeys {
				select {
				case <-ctx.Done():
					relay.Close()
					return
				default:
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
						if isConnectionError(err) {
							relay.Close()
							time.Sleep(reconnectDelay)
							break PING_LOOP
						}
						continue
					}
					fmt.Printf("sent ping to %s\n", pubkey)
					time.Sleep(time.Duration(interval) * time.Second)
				}
			}
		}

		relay.Close()
		fmt.Println("Attempting to reconnect...")
		time.Sleep(reconnectDelay)
	}
}
