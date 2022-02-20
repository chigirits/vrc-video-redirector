# vrc-video-redirector

This is an incomplete clone of the ["Jinnai System"](https://t-ne.x0.to/), the HTTP server that redirects the video URL to a playable one in VRChat for Meta Quest.
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
   --ytdlp-path value  path to yt-dlp command (default: "/usr/local/bin/yt-dlp")
   --url-root value    URL root path excluding before the domain name (for reverse proxy) (default: "/")
   --disable-cache     disable cache (default: false)
   --log-level value   log level [debug, info, warn, error, off] (default: "info")
   --help, -h          show help (default: false)
   --version, -v       print the version (default: false)
```

## Example

When assigning `https://example.com/path/to/vvr/` as root, running on port `8000` and accessing via reverse proxy

1. `wget https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -O /path/to/somewhere/yt-dlp`
2. Run `vrc-video-redirector --port 8000 --url-root /path/to/vvr/ --ytdlp-path /path/to/somewhere/yt-dlp`
3. Access to `https://example.com/path/to/vvr/www.youtube.com/watch?v=xxxxxxxxxxx`

## Notes

- Currently, only YouTube is supported.
