package main

import (
	"github.com/liaogx/douyin-mcp/configs"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net"
	"net/http"
	"strconv"
	"time"
)

func NewAppServer(service Operations, c configs.Config, token string) *http.Server {
	server := InitMCPServer(service)
	mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, MaxRequestBodyBytes: 64 << 10, PropagateRequestCancellation: true})
	return &http.Server{Addr: net.JoinHostPort(c.Host, strconv.Itoa(c.Port)), Handler: protect(c, token, SetupRoutes(service, mcpHandler)), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: c.OperationTimeout + time.Minute, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
}
