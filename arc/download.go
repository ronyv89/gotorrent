package arc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/ronyv89/gotorrent/core"
)

// parseDescPage parses the torrent description page and extracts the torrent file url
func parseDescPage(html string) (string, error) {
	// Load html response into GoQuery
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", fmt.Errorf("could not load html response into GoQuery: %v", err)
	}

	// Get the torrent file path from a "<a href=...>"" whose class starts with
	// "format-summary" and whose text contains the word "TORRENT"
	var fileURL string
	doc.Find(".format-summary ").EachWithBreak(func(i int, s *goquery.Selection) bool {
		if strings.Contains(s.Text(), "TORRENT") {
			path, ok := s.Attr("href")
			if ok {
				fileURL = baseURL + path
			}
			return false
		}
		return true
	})

	if fileURL != "" {
		return fileURL, nil
	}

	return "", fmt.Errorf("could not find a torrent file on the description page")
}

// FindAndDlFile opens the torrent description page and downloads the torrent
// file.
// A user timeout is set.
// Returns the local path of downloaded torrent file.
func FindAndDlFile(descURL string, in string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	html, _, err := core.Fetch(ctx, descURL, nil)
	if err != nil {
		return "", fmt.Errorf("error while fetching url: %v", err)
	}

	fileURL, err := parseDescPage(html)
	if err != nil {
		return "", fmt.Errorf("error while parsing torrent description page: %v", err)
	}

	client := &http.Client{
		Timeout: timeout,
	}

	filePath, err := core.DlFileWithoutChrome(fileURL, in, client)
	if err != nil {
		return "", fmt.Errorf("error while downloading torrent file: %v", err)
	}

	return filePath, nil
}
