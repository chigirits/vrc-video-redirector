package main

import (
	"bytes"
	"errors"
	"github.com/goware/urlx"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"github.com/urfave/cli/v2"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

type cacheEntry struct {
	expire     time.Time
	redirectTo string
}

var (
	port         int
	youtubeDl    string
	urlRoot      string
	logLevel     string
	cache        = make(map[string]cacheEntry)
	cacheMutex   sync.Mutex
	allowedHosts = map[string]struct{}{
		"www.youtube.com": struct{}{},
		"youtu.be":        struct{}{},
	}
)

func main() {
	app := cli.NewApp()
	app.Name = "vrc-video-redirector"
	app.Usage = "Video URL redirector for VRChat Quest"
	app.Version = "0.2.0"
	app.Flags = []cli.Flag{
		&cli.IntFlag{
			Name:        "port, p",
			Value:       8000,
			Destination: &port,
			EnvVars:     []string{"VVR_PORT"},
		},
		&cli.StringFlag{
			Name:        "youtube-dl, d",
			Value:       "/usr/bin/youtube-dl",
			Destination: &youtubeDl,
			EnvVars:     []string{"VVR_YOUTUBE_DL"},
		},
		&cli.StringFlag{
			Name:        "url-root, r",
			Value:       "/",
			Destination: &urlRoot,
			EnvVars:     []string{"VVR_URL_ROOT"},
		},
		&cli.StringFlag{
			Name:        "log-level, l",
			Value:       "info",
			Destination: &logLevel,
			EnvVars:     []string{"VVR_LOG_LEVEL"},
		},
	}
	app.Action = func(c *cli.Context) error {
		e := echo.New()
		e.Use(middleware.Logger())
		e.Use(middleware.Recover())
		e.GET(urlRoot+"*", handleRequest)
		if l, ok := parseLogLevel(c.String("log-level")); ok {
			e.Logger.SetLevel(l)
		} else {
			return errors.New("log-level must be one of [debug, info, warn, error, off]")
		}
		e.Logger.Infof("youtubeDl: %s", youtubeDl)
		e.Logger.Infof("urlRoot: %s", urlRoot)

		err := e.Start(":" + strconv.Itoa(port))
		if err != nil {
			e.Logger.Fatal(err)
		}
		return err
	}
	app.Run(os.Args)
}

func parseLogLevel(name string) (log.Lvl, bool) {
	switch name {
	case "debug":
		return log.DEBUG, true
	case "info":
		return log.INFO, true
	case "warn":
		return log.WARN, true
	case "error":
		return log.ERROR, true
	case "off":
		return log.OFF, true
	}
	return log.OFF, false
}

func handleRequest(c echo.Context) error {
	var err error
	e := c.Echo()
	e.Logger.Debugf("Path: %s", c.Param("*"))

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
	if _, ok := allowedHosts[u.Host]; !ok {
		return c.String(http.StatusBadRequest, "Bad Request")
	}

	// Lock mutex
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	// Check cache
	if cached, ok := cache[url]; ok {
		if time.Now().Before(cached.expire) {
			return c.Redirect(http.StatusFound, cached.redirectTo)
		}
		delete(cache, url)
	}

	// Resolve
	result, err := exec.Command(youtubeDl, "-g", url).Output()
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
