package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/goware/urlx"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"github.com/urfave/cli/v2"
)

type videoFormat struct {
	FormatID string `json:"format_id"`
	Ext      string `json:"ext"`
	URL      string `json:"url"`
	Vcodec   string `json:"vcodec"`
	Acodec   string `json:"acodec"`
}

type videoInfo struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Duration    int            `json:"duration"`
	WebpageURL  string         `json:"webpage_url"`
	Formats     []*videoFormat `json:"formats"`
}

type cacheEntry struct {
	expire      time.Time
	videoFormat *videoFormat
	videoInfo   *videoInfo
}

var (
	port         int
	youtubeDl    string
	urlRoot      string
	logLevel     string
	cache        = make(map[string]*cacheEntry)
	cacheMutex   sync.Mutex
	allowedHosts = map[string]struct{}{
		"www.youtube.com": {},
		"youtu.be":        {},
	}
)

func main() {
	app := cli.NewApp()
	app.Name = "vrc-video-redirector"
	app.Usage = "Video URL redirector for VRChat on Meta Quest"
	app.Version = "0.3.0"
	app.Flags = []cli.Flag{
		&cli.IntFlag{
			Name:        "port, p",
			Value:       8000,
			Destination: &port,
			Usage:       "port number of the HTTP server",
		},
		&cli.StringFlag{
			Name:        "youtube-dl, d",
			Value:       "/usr/bin/youtube-dl",
			Destination: &youtubeDl,
			Usage:       "path to youtube-dl command",
		},
		&cli.StringFlag{
			Name:        "url-root, r",
			Value:       "/",
			Destination: &urlRoot,
			Usage:       "URL root path excluding before the domain name (for reverse proxy)",
		},
		&cli.StringFlag{
			Name:        "log-level, l",
			Value:       "info",
			Destination: &logLevel,
			Usage:       "log level [debug, info, warn, error, off]",
		},
	}
	app.Action = func(c *cli.Context) error {
		e := echo.New()
		e.HideBanner = true
		e.Use(middleware.Logger())
		e.Use(middleware.Recover())
		e.Use(middleware.RequestID())
		e.HEAD(urlRoot+"*", handleRequest)
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

func resolve(url string) (*videoFormat, *videoInfo, error) {
	b, err := exec.Command(youtubeDl, "-J", url).Output()
	if err != nil {
		return nil, nil, err
	}
	var info *videoInfo
	if err := json.Unmarshal(b, &info); err != nil {
		return nil, nil, err
	}
	var second *videoFormat
	for _, f := range info.Formats {
		if !(f.Ext == "webm" || f.Ext == "mp4") {
			continue
		}
		if second == nil {
			second = f
		}
		if f.Vcodec == "none" || f.Acodec == "none" {
			continue
		}
		return f, info, nil
	}
	if second != nil {
		return second, info, nil
	}
	if len(info.Formats) == 0 {
		return nil, info, errors.New("No format")
	}
	return info.Formats[0], info, nil
}

func handleRequest(c echo.Context) error {
	var err error
	e := c.Echo()
	path := c.Param("*")
	e.Logger.Debugf("Path: %s", path)

	// Normalize URL
	u, err := urlx.Parse(path)
	if err != nil {
		e.Logger.Warnf("Failed to parse URL: %s", err.Error())
		return c.String(http.StatusBadRequest, "Bad Request")
	}
	if !strings.HasPrefix(path, u.Scheme+":") {
		u.Scheme = "https"
	}
	u.RawQuery = c.QueryString()
	url, _ := urlx.Normalize(u)
	e.Logger.Debugf("Normalized URL: %s", url)

	// Validate URL
	if _, ok := allowedHosts[u.Host]; !ok {
		e.Logger.Debugf("Host not allowed: %s", url)
		return c.String(http.StatusNotFound, "Not Found")
	}

	// Redirect to web page on Windows
	if strings.Contains(c.Request().UserAgent(), "Windows") {
		e.Logger.Debugf("Redirect to web page: %s", url)
		return c.Redirect(http.StatusFound, url)
	}

	// Lock mutex
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	// Check cache
	if cached, ok := cache[url]; ok {
		if time.Now().Before(cached.expire) {
			e.Logger.Debugf("Using cache: %s", cached.videoFormat.URL)
			return c.Redirect(http.StatusFound, cached.videoFormat.URL)
		}
		delete(cache, url)
	}

	// Resolve
	videoFormat, videoInfo, err := resolve(url)
	if err != nil {
		e.Logger.Info("Could not resolve: %s", err.Error())
		return c.Redirect(http.StatusFound, url)
	}
	e.Logger.Debugf("Resolved URL: %s", videoFormat.URL)

	// Cache
	if r, err := urlx.Parse(videoFormat.URL); err == nil {
		q := r.Query()
		// e.Logger.Debugf("Query: %#v", q)
		if 0 < len(q["expire"]) {
			if expireUnix, err := strconv.Atoi(q["expire"][0]); err == nil {
				expire := time.Unix(int64(expireUnix), 0)
				e.Logger.Debugf("Expire: %s", expire)
				cache[url] = &cacheEntry{
					expire:      expire,
					videoFormat: videoFormat,
					videoInfo:   videoInfo,
				}
			}
		}
	}

	// Response
	return c.Redirect(http.StatusFound, videoFormat.URL)
}
