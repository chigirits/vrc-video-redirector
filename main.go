package main

import (
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/goware/urlx"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"github.com/urfave/cli/v2"
)

var (
	port           int
	ytdlpPath      string
	urlRoot        string
	disableCache   bool
	logLevel       string
	cache          *Cache
	trustedDomains = map[string]struct{}{
		"www.youtube.com": {},
		"youtu.be":        {},
	}
	supportedExts = map[string]struct{}{
		"mp4": {},
		// "webm": {},
	}
	stringToLogLevel = map[string]log.Lvl{
		"debug": log.DEBUG,
		"info":  log.INFO,
		"warn":  log.WARN,
		"error": log.ERROR,
		"off":   log.OFF,
	}
)

func main() {
	app := cli.NewApp()
	app.Name = "vrc-video-redirector"
	app.Usage = "Redirect the video URL to a playable one in VRChat for Meta Quest."
	app.Version = "0.4.0"
	app.Flags = []cli.Flag{
		&cli.IntFlag{
			Name:        "port, p",
			Value:       8000,
			Destination: &port,
			Usage:       "port number of the HTTP server",
		},
		&cli.StringFlag{
			Name:        "ytdlp-path, d",
			Value:       "/usr/bin/yt-dlp",
			Destination: &ytdlpPath,
			Usage:       "path to yt-dlp command",
		},
		&cli.StringFlag{
			Name:        "url-root, r",
			Value:       "/",
			Destination: &urlRoot,
			Usage:       "URL root path excluding before the domain name (for reverse proxy)",
		},
		&cli.BoolFlag{
			Name:        "disable-cache, C",
			Value:       false,
			Destination: &disableCache,
			Usage:       "Disable cache",
		},
		&cli.StringFlag{
			Name:        "log-level, l",
			Value:       "info",
			Destination: &logLevel,
			Usage:       "log level [debug, info, warn, error, off]",
		},
	}
	app.Action = func(c *cli.Context) error {
		cache = NewCache()
		e := echo.New()
		e.HideBanner = true
		e.Use(middleware.Logger())
		e.Use(middleware.Recover())
		e.Use(middleware.RequestID())
		e.HEAD(urlRoot+"*", handleRequest)
		e.GET(urlRoot+"*", handleRequest)
		if l, ok := stringToLogLevel[strings.ToLower(logLevel)]; ok {
			e.Logger.SetLevel(l)
		} else {
			return errors.New("log-level must be one of [debug, info, warn, error, off]")
		}
		e.Logger.Infof("ytdlpPath: %s", ytdlpPath)
		e.Logger.Infof("urlRoot: %s", urlRoot)
		e.Logger.Infof("logLevel: %s", logLevel)

		err := e.Start(":" + strconv.Itoa(port))
		if err != nil {
			e.Logger.Fatal(err)
		}
		return err
	}
	app.Run(os.Args)
}

func handleRequest(c echo.Context) error {
	var err error
	e := c.Echo()
	request := c.Request()
	e.Logger.Debugf("Access: %#v", request.Header)

	thruHeader := func(options []string, name string) []string {
		if 0 < len(request.Header[name]) {
			return append(options, "--add-header", name+":"+request.Header[name][0])
		}
		return options
	}
	var options []string
	options = thruHeader(options, "User-Agent")
	// options = thruHeader(options, "X-Forwarded-For")
	// options = thruHeader(options, "X-Real-Ip")
	path := c.Param("*")

	// Normalize URL
	u, err := urlx.ParseWithDefaultScheme(path, "https")
	if err != nil {
		e.Logger.Warnf("Failed to parse URL: %s", err.Error())
		return c.String(http.StatusBadRequest, "Bad Request")
	}
	u.RawQuery = c.QueryString()
	url, _ := urlx.Normalize(u)
	e.Logger.Debugf("Normalized URL: %s", url)

	// Validate URL
	if _, ok := trustedDomains[u.Host]; !ok {
		e.Logger.Debugf("Untrusted domain: %s", url)
		return c.String(http.StatusNotFound, "Not Found")
	}

	// Redirect to web page on Windows
	if strings.Contains(request.UserAgent(), "Windows") {
		e.Logger.Debugf("Redirect to web page: %s", url)
		return c.Redirect(http.StatusFound, url)
	}

	// Lock mutex
	cache.Mutex.Lock()
	defer cache.Mutex.Unlock()

	// Check cache
	if cached, ok := cache.Load(url); ok {
		e.Logger.Debugf("Using cache: %s", cached.Format.URL)
		return c.Redirect(http.StatusFound, cached.Format.URL)
	}

	// Resolve
	videoFormat, videoInfo, err := resolve(url, options)
	if err != nil {
		e.Logger.Infof("Could not resolve: %s", err.Error())
		return c.Redirect(http.StatusFound, url)
	}
	e.Logger.Debugf("Resolved URL: %s", videoFormat.URL)

	// Cache
	if !disableCache {
		cache.Store(videoFormat, videoInfo, url)
	}

	// Response
	return c.Redirect(http.StatusFound, videoFormat.URL)
}
