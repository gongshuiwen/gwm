package main

import (
	"context"
	"os"

	"gwm/internal/app"
)

func main() {
	os.Exit(app.New().Run(context.Background(), os.Args[1:]))
}
