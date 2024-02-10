package gateway

import (
	"bufio"
	"encoding/base64"
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

	user, err := s.authHandler(username, password)
	if err != nil {
		ctx.Response.SetStatusCode(fasthttp.StatusUnauthorized)
		ctx.Response.ConnectionClose()
		return
	}

	if string(ctx.Method()) == "CONNECT" {
		s.handleConnect(ctx, user)
	} else {
		s.handleHTTP(ctx, user)
	}
}

func (s *Proxy) handleConnect(ctx *fasthttp.RequestCtx, user *svc.User) {
	ctx.Response.SetStatusCode(fasthttp.StatusOK)
	ctx.Response.SetBody(nil)
	host := string(ctx.Host())

	ctx.Hijack(func(userConn net.Conn) {
		s.sessionHandler(user, svc.Destination(host), s.config.Region, userConn)
	})
}

func (s *Proxy) handleHTTP(ctx *fasthttp.RequestCtx, user *svc.User) {
	host := string(ctx.Request.URI().Host())
	if !regexp.MustCompile(`:\d+$`).MatchString(host) {
		host += ":80"
	}

	pipeConn, txConn := net.Pipe()
	defer pipeConn.Close()

	bufWriter := bufio.NewWriter(pipeConn)
	defer bufWriter.Flush()

	go s.sessionHandler(user, svc.Destination(host), s.config.Region, txConn)

	if err := streamRequest(ctx, bufWriter); err != nil {
		ctx.Response.SetStatusCode(fasthttp.StatusInternalServerError)
		return
	}

	if err := readResponse(ctx, bufio.NewReader(pipeConn)); err != nil {
		ctx.Response.SetStatusCode(fasthttp.StatusServiceUnavailable)
		return
	}
}

func streamRequest(ctx *fasthttp.RequestCtx, w *bufio.Writer) error {
	defer w.Flush()

	stripProxyHeaders(&ctx.Request)

	if err := ctx.Request.Write(w); err != nil {
		return err
	}
	return nil
}

func readResponse(ctx *fasthttp.RequestCtx, rd *bufio.Reader) error {
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	if err := resp.Read(rd); err != nil {
		return err
	}

	resp.CopyTo(&ctx.Response)
	return nil
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
