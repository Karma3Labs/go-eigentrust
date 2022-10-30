package main

import (
	"github.com/eigentrust-io/go-eigentrust/internal/playground"
	"github.com/gin-gonic/gin"
)

func main() {
	gr := gin.Default()
	// TODO(ek): Fix this so it can run when installed; see embed module.
	gr.LoadHTMLGlob("templates/*")
	playground.AddRoutes(gr)
	_ = gr.Run()
}
