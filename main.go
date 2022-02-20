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
	port           int
	ytdlpPath      string
	urlRoot        string
	disableCache   = true
	logLevel       string
	cache          = make(map[string]*cacheEntry)
	cacheMutex     sync.Mutex
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
	app.Usage = "Video URL redirector for VRChat on Meta Quest"
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

func resolve(url string, options []string) (*videoFormat, *videoInfo, error) {
	options = append(options, "-J", url)
	b, err := exec.Command(ytdlpPath, options...).Output()
	if err != nil {
		return nil, nil, err
	}
	var info *videoInfo
	if err := json.Unmarshal(b, &info); err != nil {
		return nil, nil, err
	}
	var first *videoFormat
	var second *videoFormat
	for _, f := range info.Formats {
		if _, ok := supportedExts[f.Ext]; !ok {
			continue
		}
		if second == nil {
			second = f
		}
		if f.Vcodec == "none" || f.Acodec == "none" {
			continue
		}
		first = f
	}
	if first != nil {
		return first, info, nil
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
	videoFormat, videoInfo, err := resolve(url, options)
	if err != nil {
		e.Logger.Infof("Could not resolve: %s", err.Error())
		return c.Redirect(http.StatusFound, url)
	}
	e.Logger.Debugf("Resolved URL: %s", videoFormat.URL)

	// Cache
	if r, err := urlx.Parse(videoFormat.URL); err == nil {
		q := r.Query()
		if !disableCache && 0 < len(q["expire"]) {
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
