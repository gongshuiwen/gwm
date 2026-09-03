// Command gwm provides the GWM command-line interface.
package main

import (
	"context"
	"os"

	"github.com/gongshuiwen/gwm/internal/app"
)

func main() {
	os.Exit(app.New().Run(context.Background(), os.Args[1:]))
}
