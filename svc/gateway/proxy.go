package gateway

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/bacv/kingip/svc"
	"github.com/valyala/fasthttp"
)

type ProxyConfig struct {
	Addr   net.Addr
	Region svc.Region
}

type Proxy struct {
	config         ProxyConfig
	authHandler    svc.GatewayAuthHandleFunc
	sessionHandler svc.GatewaySessionHandleFunc
}

func NewProxyServer(
	config ProxyConfig,
	authHandler svc.GatewayAuthHandleFunc,
	sessionHandler svc.GatewaySessionHandleFunc,
) (*Proxy, error) {
	return &Proxy{
		config:         config,
		authHandler:    authHandler,
		sessionHandler: sessionHandler,
	}, nil
}

func (s *Proxy) ListenUser() error {
	return fasthttp.ListenAndServe(s.config.Addr.String(), s.handleRequest)
}

func (s *Proxy) handleRequest(ctx *fasthttp.RequestCtx) {
	username, password, ok := parseBasicAuth(ctx)
	if !ok {
		ctx.Response.SetStatusCode(fasthttp.StatusUnauthorized)
		ctx.Response.ConnectionClose()
		return
	}

	err := s.authHandler(username, password)
	if err != nil {
		ctx.Response.SetStatusCode(fasthttp.StatusUnauthorized)
		ctx.Response.ConnectionClose()
		return
	}

	if string(ctx.Method()) == "CONNECT" {
		s.handleConnect(ctx)
	} else {
		s.handleHTTP(ctx)
	}
}

func (s *Proxy) handleConnect(ctx *fasthttp.RequestCtx) {
	ctx.Response.SetStatusCode(fasthttp.StatusOK)
	ctx.Response.SetBody(nil)

	ctx.Hijack(func(userConn net.Conn) {
		s.sessionHandler(s.config.Region, userConn)
	})
}

func (s *Proxy) handleHTTP(ctx *fasthttp.RequestCtx) {
	destHost := string(ctx.Request.URI().Host())

	re := regexp.MustCompile(`:\d+$`)
	if !re.MatchString(destHost) {
		destHost = fmt.Sprintf("%s:80", destHost)
	}

	destConn, err := net.Dial("tcp", destHost)
	if err != nil {
		ctx.Logger().Printf("Error connecting to destination [%s]: %v", destHost, err)
		ctx.Response.SetStatusCode(fasthttp.StatusServiceUnavailable)
		return
	}
	defer destConn.Close()

	bufWriter := bufio.NewWriter(destConn)
	defer bufWriter.Flush()

	stripProxyHeaders(&ctx.Request)

	err = ctx.Request.Write(bufWriter)
	if err != nil {
		ctx.Logger().Printf("Error streaming request to destination: %v", err)
		ctx.Response.SetStatusCode(fasthttp.StatusInternalServerError)
		return
	}

	err = bufWriter.Flush()
	if err != nil {
		ctx.Logger().Printf("Error flushing buffer to destination: %v", err)
		ctx.Response.SetStatusCode(fasthttp.StatusInternalServerError)
		return
	}

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	err = resp.Read(bufio.NewReader(destConn))
	if err != nil {
		ctx.Logger().Printf("Error reading response from destination: %v", err)
		ctx.Response.SetStatusCode(fasthttp.StatusServiceUnavailable)
		return
	}

	resp.CopyTo(&ctx.Response)
}

func parseBasicAuth(ctx *fasthttp.RequestCtx) (username, password string, ok bool) {
	auth := string(ctx.Request.Header.Peek("Proxy-Authorization"))
	if !strings.HasPrefix(auth, "Basic ") {
		return "", "", false
	}

	decoded, err := base64.StdEncoding.DecodeString(auth[6:])
	if err != nil {
		return "", "", false
	}

	creds := strings.SplitN(string(decoded), ":", 2)
	if len(creds) != 2 {
		return "", "", false
	}

	return creds[0], creds[1], true
}

func stripProxyHeaders(req *fasthttp.Request) {
	proxyHeaders := []string{
		"Proxy-Authorization",
		"Proxy-Connection",
	}

	for _, header := range proxyHeaders {
		req.Header.Del(header)
	}
}
