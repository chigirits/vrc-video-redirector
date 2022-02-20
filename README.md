# vrc-video-redirector

This is an HTTP server that redirects the video URL to a playable one in VRChat for Meta Quest.
This is an incomplete clone of the ["Jinnai System"](https://t-ne.x0.to/).
You can use it on your own server without worrying about the status of the original Jinnai System.

## Requirements

- [yt-dlp](https://github.com/yt-dlp/yt-dlp)

## Usage

```
NAME:
   vrc-video-redirector - Video URL redirector for VRChat on Meta Quest

USAGE:
   main [global options] command [command options] [arguments...]

VERSION:
   0.4.0

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --port value        port number of the HTTP server (default: 8000)
   --ytdlp-path value  path to yt-dlp command (default: "/usr/bin/yt-dlp")
   --url-root value    URL root path excluding before the domain name (for reverse proxy) (default: "/")
   --disable-cache     Disable cache (default: false)
   --log-level value   log level [debug, info, warn, error, off] (default: "info")
   --help, -h          show help (default: false)
   --version, -v       print the version (default: false)
```
