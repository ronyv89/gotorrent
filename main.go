package main

import (
	"fmt"

	"github.com/ronyv89/gotorrent/gotorrent"
)

func main() {
	sources := []string{"otts"}
	res := gotorrent.TorrentSearch(sources, "Movies", "Barbie 2160p")
	for _, torrent := range res {
		fmt.Println(torrent)
	}
}
