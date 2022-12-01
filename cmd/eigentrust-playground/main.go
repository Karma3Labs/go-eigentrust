package main

import (
	"github.com/gin-gonic/gin"
	"k3l.io/go-eigentrust/internal/playground"
)

func main() {
	gr := gin.Default()
	// TODO(ek): Fix this so it can run when installed; see embed module.
	gr.LoadHTMLGlob("templates/*")
	playground.AddRoutes(gr)
	_ = gr.Run()
}
