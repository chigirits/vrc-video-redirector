package main

import (
	"bytes"
	"github.com/goware/urlx"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"net/http"
	"os/exec"
	"strconv"
	"time"
)

const (
	YOUTUBEDL_PATH = "youtube-dl"
	SERVER_PORT    = ":8000"
	LOG_LEVEL      = log.DEBUG
)

type cacheEntry struct {
	expire     time.Time
	redirectTo string
}

var (
	cache       = make(map[string]cacheEntry)
	allowdHosts = map[string]struct{}{
		"www.youtube.com": struct{}{},
		"youtu.be":        struct{}{},
	}
)

func main() {
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.GET("/*", handleRequest)
	e.Logger.SetLevel(LOG_LEVEL)
	e.Logger.Fatal(e.Start(SERVER_PORT))
}

func handleRequest(c echo.Context) error {
	var err error
	e := c.Echo()

	// Normalize URL
	u, err := urlx.Parse(c.Param("*"))
	if err != nil {
		e.Logger.Warnf("Failed to parse URL: %s", err.Error())
		return c.String(http.StatusBadRequest, "Bad Request")
	}
	u.RawQuery = c.QueryString()
	url, _ := urlx.Normalize(u)
	e.Logger.Debugf("Normalized URL: %s", url)

	// Validate URL
	if _, ok := allowdHosts[u.Host]; !ok {
		return c.String(http.StatusBadRequest, "Bad Request")
	}

	// Check cache
	if cached, ok := cache[url]; ok {
		if time.Now().Before(cached.expire) {
			return c.Redirect(http.StatusFound, cached.redirectTo)
		}
		delete(cache, url)
	}

	// Resolve
	result, err := exec.Command(YOUTUBEDL_PATH, "-g", url).Output()
	if err != nil {
		return c.String(http.StatusBadGateway, "502 Bad Gateway")
	}
	lines := bytes.SplitN(result, []byte{'\n'}, 2)
	redirectTo := string(lines[0])
	e.Logger.Debugf("Resolved URL: %s", redirectTo)

	// Cache
	if r, err := urlx.Parse(redirectTo); err == nil {
		q := r.Query()
		e.Logger.Debugf("Query: %#v", q)
		if 0 < len(q["expire"]) {
			if expireUnix, err := strconv.Atoi(q["expire"][0]); err == nil {
				expire := time.Unix(int64(expireUnix), 0)
				e.Logger.Debugf("Expire: %s", expire)
				cache[url] = cacheEntry{
					expire:     expire,
					redirectTo: redirectTo,
				}
			}
		}
	}

	// Response
	return c.Redirect(http.StatusFound, redirectTo)
}
