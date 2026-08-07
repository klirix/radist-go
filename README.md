# radist-server (Go)

Server-side Radist SDK for creating P2P calls and SFU rooms from trusted environments.

## Requirements

Go 1.21 or later.

## Installation

```sh
go get github.com/klirix/radist-go
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
import radist "github.com/klirix/radist-go"

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

### Persistent Spaces

Spaces are reusable meeting rooms with a stable join URL, unlike P2P calls
and SFU rooms which are created fresh each time.

```go
space, err := client.CreateSpace(ctx, radist.CreateSpaceOptions{
    Name:       "Team Standup",
    AccessType: radist.SpaceAccessPublic,
})
if err != nil {
    log.Fatal(err)
}

fmt.Println(space.Space.ID, space.ParticipantURL, space.HostToken)

spaces, err := client.ListSpaces(ctx, nil)
if err != nil {
    log.Fatal(err)
}
for _, s := range spaces {
    fmt.Println(s.ID, s.Slug, s.ParticipantURL)
}
```

`SpaceCreated.HostToken` and `SpaceCreated.ParticipantURL` are ready to hand
to the host and to participants respectively; use `RotateSpaceHostToken` to
invalidate and reissue the host token later.

```go
tok, err := client.RotateSpaceHostToken(ctx, space.Space.ID, nil)
if err != nil {
    log.Fatal(err)
}
fmt.Println(tok.HostURL, tok.HostToken)
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
