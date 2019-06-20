# msvc

Framework for building micro-services in Go.

## Example

```go
package main

import (

"context"
"fmt"
"github.com/arikkfir/msvc"
"github.com/arikkfir/msvc/daemon/http"
"github.com/arikkfir/msvc/middleware"
"os"
)

type Config struct {
	HTTP     http.Config
	Metrics  middleware.MetricsConfig
}

type UsersService struct {}

type GetUsersRequest struct{}
type User struct {Name string `json:"name"`}
type GetUsersResponse struct {
    Users []User
}
func (s *UsersService) GetUsers(ctx context.Context, r *GetUsersRequest) (*GetUsersResponse, error) {
    return &GetUsersResponse{
        Users:[]User{
            {Name: "Joe"},
            {Name: "Jack"},
        },
    }, nil
}

func main() {

	// Setup configuration
	config := Config{}
	config.HTTP.Port = 3001
	config.Metrics.Port = 3002

	// Create the service
	ms, err := msvc.New("myService", &config)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	// Create the service
	service := &UsersService{}

	// Add middleware
	ms.AddMiddleware(middleware.MethodDuration)
	ms.AddMiddleware(middleware.Logging)

	// Add service methods
	getUsersAdapter := ms.AddMethod("GetUsers", service.GetUsers)

	// Add daemons
	ms.AddDaemon(middleware.NewMetricsServer(&config.Metrics))
	ms.AddDaemon(http.NewHTTPServer(
		ms,
		&http.Config{Port:3000},
		map[string]interface{}{
			"/v1": map[string]interface{}{
				"users": map[string]interface{}{
					"GET":  http.NewHandler(getUsersAdapter),
				},
			},
		},
	))

	// Run micro-service
	ms.Run()
}
```