package main

import (
	"encoding/json"
	"errors"
	"os/exec"
)

type VideoFormat struct {
	FormatID string `json:"format_id"`
	Ext      string `json:"ext"`
	URL      string `json:"url"`
	Vcodec   string `json:"vcodec"`
	Acodec   string `json:"acodec"`
}

type VideoInfo struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Duration    int            `json:"duration"`
	WebpageURL  string         `json:"webpage_url"`
	Formats     []*VideoFormat `json:"formats"`
}

func resolve(url string, options []string) (*VideoFormat, *VideoInfo, error) {
	options = append(options, "-J", url)
	b, err := exec.Command(ytdlpPath, options...).Output()
	if err != nil {
		return nil, nil, err
	}
	var info *VideoInfo
	if err := json.Unmarshal(b, &info); err != nil {
		return nil, nil, err
	}
	var first *VideoFormat
	var second *VideoFormat
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
