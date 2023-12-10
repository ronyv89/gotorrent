package gotorrent

import (
	"fmt"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/onrik/logrus/filename"
	log "github.com/sirupsen/logrus"

	"github.com/ronyv89/gotorrent/arc"
	"github.com/ronyv89/gotorrent/otts"
	"github.com/ronyv89/gotorrent/tpb"
	"github.com/ronyv89/gotorrent/ygg"
)

// lineBreak sets the OS dependent line break (initialized in init())
var lineBreak string

// sources maps source short names to real names
var sources = map[string]string{
	"arc":  "Archive",
	"tpb":  "The Pirate Bay",
	"otts": "1337x",
	"ygg":  "Ygg Torrent",
}

// isVerbose is used to switch debugging on or off
var isVerbose bool

// ft is the final torrent the user wants to download
var ft Torrent

// torrent contains meta information about the torrent
type Torrent struct {
	FileURL string
	Magnet  string
	// Description url containing more info about the torrent including the torrent file address
	DescURL  string
	Name     string
	Size     string
	Seeders  int
	Leechers int
	// Date of upload
	UplDate string
	// Website the torrent is coming from
	Source string
	// Local path where torrent was saved
	FilePath string
}

// torListAndHTTPClient contains the torrents found and the http client
type torListAndHTTPClient struct {
	torList    []Torrent
	httpClient *http.Client
}

// search represents the user search
type Search struct {
	In              string
	Out             []Torrent
	SourcesToLookup []string
	HttpClient      *http.Client
}

// cleanIn cleans the user search input
func (s *Search) CleanIn() error {
	// Clean user input by removing useless spaces
	strings.TrimSpace(s.In)

	// If user input is empty raise an error
	if s.In == "" {
		return fmt.Errorf("User input should not be empty")
	}

	return nil
}

// sortOut sorts torrents list based on number of seeders (top down)
func (s *Search) SortOut() {
	sort.Slice(s.Out, func(i, j int) bool {
		return s.Out[i].Seeders > s.Out[j].Seeders
	})
}

// getTorrentFile retrieves and displays torrent file to user.
// TODO(juliensalinas): pass a proper context.Context object instead
// of a mere timeout.
func getTorrentFile(userID, in string, userPass string,
	timeout time.Duration, httpClient *http.Client) {
	var err error
	switch ft.Source {
	case "arc":
		log.WithFields(log.Fields{
			"sourceToSearch": "arc",
		}).Debug("Download torrent file")
		ft.FilePath, err = arc.FindAndDlFile(ft.DescURL, in, timeout)
	case "ygg":
		log.WithFields(log.Fields{
			"sourceToSearch": "ygg",
		}).Debug("Download torrent file")
		ft.FilePath, err = ygg.FindAndDlFile(
			ft.DescURL, in, userID, userPass, timeout, httpClient)
	}
	if err != nil {
		fmt.Println("Could not retrieve the torrent file (see logs for more details).")
		log.WithFields(log.Fields{
			"descURL": ft.DescURL,
			"error":   err,
		}).Fatal("Could not retrieve the torrent file")
	}
}

// rmDuplicates removes duplicates from slice
func rmDuplicates(elements []string) []string {
	encountered := map[string]bool{}

	// Create a map of all unique elements.
	for v := range elements {
		encountered[elements[v]] = true
	}

	// Place all keys from the map into a slice.
	result := []string{}
	for key := range encountered {
		result = append(result, key)
	}
	return result
}

// setLogger sets various logging parameters
func setLogger(isVerbose bool) {
	// If verbose, set logger to debug, otherwise display errors only
	if isVerbose {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.ErrorLevel)
	}

	// Log as standard text
	log.SetFormatter(&log.TextFormatter{})

	// Log as JSON instead of the default ASCII formatter
	// log.SetFormatter(&log.JSONFormatter{})

	// Log filename and line number.
	// Should be removed from production because adds a performance cost.
	log.AddHook(filename.NewHook())
}

func Init() {
	// Set custom line break in order for the script to work on any OS
	if runtime.GOOS == "windows" {
		lineBreak = "\r\n"
	} else {
		lineBreak = "\n"
	}
}

func TorrentSearch(searchSources []string, category string, query string) []Torrent {
	Init()
	cleanedUsrSourcesSlc := rmDuplicates(searchSources)
	for _, usrSource := range cleanedUsrSourcesSlc {
		if usrSource == "all" {
			cleanedUsrSourcesSlc = []string{"arc", "tpb", "otts", "ygg"}
			break
		}
		if usrSource != "arc" && usrSource != "tpb" && usrSource != "otts" && usrSource != "ygg" {
			fmt.Printf("This website is not correct: %v%v", usrSource, lineBreak)
			log.WithFields(log.Fields{
				"sourcesList": cleanedUsrSourcesSlc,
				"wrongSource": usrSource,
			}).Fatal("Unknown source in user sources list")
		}
	}
	s := Search{
		In:              query,
		SourcesToLookup: cleanedUsrSourcesSlc,
	}

	// Clean user input
	err := s.CleanIn()
	if err != nil {
		fmt.Println("Could not process your input (see logs for more details).")
		log.WithFields(log.Fields{
			"input": s.In,
			"error": err,
		}).Fatal("Could not clean user input")
	}
	// Channels for results
	arcTorListCh := make(chan []Torrent)
	tpbTorListCh := make(chan []Torrent)
	ottsTorListCh := make(chan []Torrent)
	yggTorListAndHTTPClientCh := make(chan torListAndHTTPClient)

	// Channels for errors
	arcSearchErrCh := make(chan error)
	tpbSearchErrCh := make(chan error)
	ottsSearchErrCh := make(chan error)
	yggSearchErrCh := make(chan error)

	timeout := time.Duration(20000 * 1000 * 1000)
	for _, source := range s.SourcesToLookup {
		switch source {
		// User wants to search arc
		case "arc":
			go func() {
				log.WithFields(log.Fields{
					"input":          s.In,
					"sourceToSearch": "arc",
				}).Debug("Start search goroutine")
				arcTorrents, err := arc.Lookup(s.In, category, timeout)
				if err != nil {
					arcSearchErrCh <- err
					return
				}
				var torList []Torrent
				for _, arcTorrent := range arcTorrents {
					t := Torrent{
						DescURL:  arcTorrent.DescURL,
						Name:     arcTorrent.Name,
						Size:     "Unknown",
						Leechers: -1,
						Seeders:  -1,
						Source:   "arc",
					}
					torList = append(torList, t)
				}
				arcTorListCh <- torList
			}()

		// User wants to search tpb
		case "tpb":
			go func() {
				log.WithFields(log.Fields{
					"input":          s.In,
					"sourceToSearch": "tpb",
				}).Debug("Start search goroutine")
				tpbTorrents, err := tpb.Lookup(s.In, timeout)
				if err != nil {
					tpbSearchErrCh <- err
					return
				}
				var torList []Torrent
				for _, tpbTorrent := range tpbTorrents {
					t := Torrent{
						Magnet:   tpbTorrent.Magnet,
						Name:     tpbTorrent.Name,
						Size:     tpbTorrent.Size,
						UplDate:  tpbTorrent.UplDate,
						Leechers: tpbTorrent.Leechers,
						Seeders:  tpbTorrent.Seeders,
						Source:   "tpb",
					}
					torList = append(torList, t)
				}
				tpbTorListCh <- torList
			}()
		// User wants to search otts
		case "otts":
			go func() {
				log.WithFields(log.Fields{
					"input":          s.In,
					"sourceToSearch": "otts",
				}).Debug("Start search goroutine")
				ottsTorrents, err := otts.Lookup(s.In, category, timeout)
				if err != nil {
					ottsSearchErrCh <- err
					return
				}
				var torList []Torrent
				for _, ottsTorrent := range ottsTorrents {
					fmt.Println(ottsTorrent)
					t := Torrent{
						DescURL:  ottsTorrent.DescURL,
						Name:     ottsTorrent.Name,
						Size:     ottsTorrent.Size,
						UplDate:  ottsTorrent.UplDate,
						Leechers: ottsTorrent.Leechers,
						Seeders:  ottsTorrent.Seeders,
						Source:   "otts",
					}
					torList = append(torList, t)
				}
				ottsTorListCh <- torList
			}()
		// User wants to search ygg
		case "ygg":
			go func() {
				log.WithFields(log.Fields{
					"input":          s.In,
					"sourceToSearch": "ygg",
				}).Debug("Start search goroutine")
				yggTorrents, httpClient, err := ygg.Lookup(s.In, category, timeout)
				if err != nil {
					yggSearchErrCh <- err
					return
				}
				var torList []Torrent
				for _, yggTorrent := range yggTorrents {
					t := Torrent{
						DescURL:  yggTorrent.DescURL,
						Name:     yggTorrent.Name,
						Size:     yggTorrent.Size,
						UplDate:  yggTorrent.UplDate,
						Leechers: yggTorrent.Leechers,
						Seeders:  yggTorrent.Seeders,
						Source:   "ygg",
					}
					torList = append(torList, t)
				}

				yggTorListAndHTTPClient := torListAndHTTPClient{torList, httpClient}
				yggTorListAndHTTPClientCh <- yggTorListAndHTTPClient
			}()
		}
	}

	// Initialize search errors
	var arcSearchErr, tpbSearchErr, ottsSearchErr, yggSearchErr error

	// Gather all goroutines results
	for _, source := range s.SourcesToLookup {
		switch source {
		case "arc":
			// Get results or error from arc
			select {
			case arcSearchErr = <-arcSearchErrCh:
				fmt.Printf("An error occured during search on %v%v", sources["arc"], lineBreak)
				log.WithFields(log.Fields{
					"input": s.In,
					"error": arcSearchErr,
				}).Error("The arc search goroutine broke")
			case arcTorList := <-arcTorListCh:
				s.Out = append(s.Out, arcTorList...)
				log.WithFields(log.Fields{
					"input":          s.In,
					"sourceToSearch": "arc",
				}).Debug("Got search results from goroutine")
			}
		case "tpb":
			// Get results or error from tpb
			select {
			case tpbSearchErr = <-tpbSearchErrCh:
				fmt.Printf("An error occured during search on %v%v", sources["tpb"], lineBreak)
				log.WithFields(log.Fields{
					"input": s.In,
					"error": tpbSearchErr,
				}).Error("The tpb search goroutine broke")
			case tpbTorList := <-tpbTorListCh:
				s.Out = append(s.Out, tpbTorList...)
				log.WithFields(log.Fields{
					"input":          s.In,
					"sourceToSearch": "tpb",
				}).Debug("Got search results from goroutine")
			}
		case "otts":
			// Get results or error from otts
			select {
			case ottsSearchErr = <-ottsSearchErrCh:
				fmt.Printf("An error occured during search on %v%v", sources["otts"], lineBreak)
				log.WithFields(log.Fields{
					"input": s.In,
					"error": ottsSearchErr,
				}).Error("The otts search goroutine broke")
			case ottsTorList := <-ottsTorListCh:
				s.Out = append(s.Out, ottsTorList...)
				log.WithFields(log.Fields{
					"input":          s.In,
					"sourceToSearch": "otts",
				}).Debug("Got search results from goroutine")
			}
		case "ygg":
			// Get results or error from ygg
			select {
			case yggSearchErr = <-yggSearchErrCh:
				fmt.Printf("An error occured during search on %v%v", sources["ygg"], lineBreak)
				log.WithFields(log.Fields{
					"input": s.In,
					"error": yggSearchErr,
				}).Error("The ygg search goroutine broke")
			case yggTorListAndHTTPClient := <-yggTorListAndHTTPClientCh:
				s.Out = append(s.Out, yggTorListAndHTTPClient.torList...)
				s.HttpClient = yggTorListAndHTTPClient.httpClient
				log.WithFields(log.Fields{
					"input":          s.In,
					"sourceToSearch": "ygg",
				}).Debug("Got search results from goroutine")
			}
		}
	}
	// Stop the program only if all goroutines returned an error
	if arcSearchErr != nil && tpbSearchErr != nil && ottsSearchErr != nil && yggSearchErr != nil {
		fmt.Println("All searches returned an error.")
		log.WithFields(log.Fields{
			"input": s.In,
			"error": err,
		}).Fatal("All searches broke")
	}
	return s.Out
}
