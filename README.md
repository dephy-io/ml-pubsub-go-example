## Run subscriber:

```bash
go run ./subscriber/main.go <relay_endpoint> <hex secret key> 
```

Example:

```bash
go run ./subscriber/main.go wss://relay-for-demo.dephy.dev a92947e0790e463ef74c8fe1ba6753a4f2d54b202c6e51bcd31fcb8aab4b7c5b
```

## Run publisher:

```bash
go run ./publisher/main.go <relay_endpoint> <public_keys json file> <interval>
```

Example:

```bash
go run ./publisher/main.go wss://relay-for-demo.dephy.dev public_keys.json 10
```
