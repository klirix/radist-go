# radist-server (Go)

Server-side Radist SDK for creating P2P calls and SFU rooms from trusted environments.

## Requirements

Go 1.21 or later.

## Installation

```sh
go get github.com/klirix/radist/sdks/go-server
```

## Authentication

`Client` resolves the secret token and project ID from constructor options first, then from environment variables.

| Config        | Option              | Environment variable    |
|---------------|---------------------|-------------------------|
| Secret token  | `NewClient(token)`  | `RADIST_KEY`            |
| Project ID    | `WithProjectID(id)` | `RADIST_PROJECT_ID`     |
| API base URL  | `WithAPIBaseURL(u)` | `RADIST_API_BASE_URL`   |

If no secret token is present, `NewClient` returns an error immediately. If no project ID is present, request methods return an error before making a request.

## Usage

```go
import radist "github.com/klirix/radist/sdks/go-server"

client, err := radist.NewClient(os.Getenv("RADIST_KEY"),
    radist.WithProjectID(os.Getenv("RADIST_PROJECT_ID")),
)
if err != nil {
    log.Fatal(err)
}

conn, err := client.CreateP2PConnection(ctx, nil)
if err != nil {
    log.Fatal(err)
}

hostToken := conn.CallTokens[0]
guestToken := conn.CallTokens[1]
fmt.Println(conn.CallID, hostToken, guestToken)
```

### SFU rooms

```go
room, err := client.CreateRoom(ctx, nil)
if err != nil {
    log.Fatal(err)
}

tok, err := client.MintConnectionToken(ctx, radist.MintConnectionTokenOptions{
    RoomID: room.RoomID,
})
```

### P2P additional tokens

```go
tok, err := client.MintConnectionToken(ctx, radist.MintConnectionTokenOptions{
    CallID: conn.CallID,
})
```

## Error handling

API errors implement the `error` interface and can be inspected as `*radist.APIError`:

```go
var apiErr *radist.APIError
if errors.As(err, &apiErr) {
    fmt.Println(apiErr.Status, apiErr.Message)
}
```

## Tests

```sh
go test ./...
```
