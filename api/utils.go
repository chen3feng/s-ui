package api

import (
	"net"
	"net/http"
	"strings"

	"github.com/alireza0/s-ui/logger"

	"github.com/gin-gonic/gin"
)

type Msg struct {
	Success bool        `json:"success"`
	Msg     string      `json:"msg"`
	Obj     interface{} `json:"obj"`
}

var privateCIDRs = []*net.IPNet{
	mustCIDR("10.0.0.0/8"),
	mustCIDR("172.16.0.0/12"),
	mustCIDR("192.168.0.0/16"),
	mustCIDR("127.0.0.0/8"),
	mustCIDR("::1/128"),
	mustCIDR("fc00::/7"),
	mustCIDR("fe80::/10"),
}

func mustCIDR(s string) *net.IPNet {
	_, cidr, err := net.ParseCIDR(s)
	if err != nil {
		panic("invalid CIDR: " + s)
	}
	return cidr
}

func isPrivateIP(ip net.IP) bool {
	for _, cidr := range privateCIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

func getRemoteIp(c *gin.Context) string {
	// Walk X-Forwarded-For from right to left, return the first public IP.
	// This prevents spoofing by untrusted clients who may inject fake
	// leftmost entries. The rightmost public IP is the actual client
	// as appended by the outermost trusted proxy (or the client itself).
	value := c.GetHeader("X-Forwarded-For")
	if value != "" {
		ips := strings.Split(value, ",")
		for i := len(ips) - 1; i >= 0; i-- {
			ipStr := strings.TrimSpace(ips[i])
			ip := net.ParseIP(ipStr)
			if ip != nil && !isPrivateIP(ip) {
				return ipStr
			}
		}
		// All IPs in chain are private — fall through to RemoteAddr
	}
	addr := c.Request.RemoteAddr
	ip, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return ip
}

func getHostname(c *gin.Context) string {
	host := c.Request.Host
	if strings.Contains(host, ":") {
		host, _, _ = net.SplitHostPort(c.Request.Host)
		if strings.Contains(host, ":") {
			host = "[" + host + "]"
		}
	}
	return host
}

func jsonMsg(c *gin.Context, msg string, err error) {
	jsonMsgObj(c, msg, nil, err)
}

func jsonObj(c *gin.Context, obj interface{}, err error) {
	jsonMsgObj(c, "", obj, err)
}

func jsonMsgObj(c *gin.Context, msg string, obj interface{}, err error) {
	m := Msg{
		Obj: obj,
	}
	if err == nil {
		m.Success = true
		if msg != "" {
			m.Msg = msg
		}
	} else {
		m.Success = false
		m.Msg = msg + ": " + err.Error()
		logger.Warning("failed :", err)
	}
	c.JSON(http.StatusOK, m)
}

func pureJsonMsg(c *gin.Context, success bool, msg string) {
	if success {
		c.JSON(http.StatusOK, Msg{
			Success: true,
			Msg:     msg,
		})
	} else {
		c.JSON(http.StatusOK, Msg{
			Success: false,
			Msg:     msg,
		})
	}
}

func checkLogin(c *gin.Context) {
	if !IsLogin(c) {
		if c.GetHeader("X-Requested-With") == "XMLHttpRequest" {
			pureJsonMsg(c, false, "Invalid login")
		} else {
			c.Redirect(http.StatusTemporaryRedirect, "/login")
		}
		c.Abort()
	} else {
		c.Next()
	}
}
