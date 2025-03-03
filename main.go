package main

import (
	"fmt"

	"github.com/nbd-wtf/go-nostr/nip19"
)

func main() {
	sk := "a92947e0790e463ef74c8fe1ba6753a4f2d54b202c6e51bcd31fcb8aab4b7c5b"
	// fmt.Println("sk:", sk)
	// fmt.Println("pk:", pk)
	// pkBytes, _ := hex.DecodeString(pk)
	nsec, _ := nip19.EncodePrivateKey(sk)
	fmt.Println("nsec:", nsec)
}
