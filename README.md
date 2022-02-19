# vrc-video-redirector

**WORK IN PROGRESS**

## Requirement

- [yt-dlp](https://github.com/yt-dlp/yt-dlp) (instead of youtube-dl)

## Usage

```
NAME:
   vrc-video-redirector - Video URL redirector for VRChat on Meta Quest

USAGE:
   main [global options] command [command options] [arguments...]

VERSION:
   0.3.0

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --port value        port number of the HTTP server (default: 8000)
   --youtube-dl value  path to youtube-dl command (default: "/usr/bin/youtube-dl")
   --url-root value    URL root path excluding before the domain name (for reverse proxy) (default: "/")
   --log-level value   log level [debug, info, warn, error, off] (default: "info")
   --help, -h          show help (default: false)
   --version, -v       print the version (default: false)
```
