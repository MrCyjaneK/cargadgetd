package main

import (
	"log"
	"time"

	"github.com/cargadgetd/cargadgetd/internal/session"
	"github.com/cargadgetd/cargadgetd/internal/shell"
)

func publishNowPlaying(md *session.MD, ui *shell.Shell) {
	go func() {
		loaded := make(chan struct{})
		go func() {
			ui.WaitLoad(2 * time.Minute)
			close(loaded)
		}()
		select {
		case <-md.Done():
			return
		case <-loaded:
		}
		var (
			sentMeta bool
			meta     session.PlaybackMeta
			cleared  bool
			noted    bool
		)
		publish := func() {
			if md.MediaPlaybackCh() == 0 {
				if !noted {
					log.Printf("media playback: head unit did not advertise it")
					noted = true
				}
				return
			}
			if !noted {
				log.Printf("media playback channel open")
				noted = true
			}
			np, live := ui.NowPlaying()
			if !live {
				if cleared {
					return
				}
				if err := md.SendPlaybackStatus(session.PlaybackStopped, "", 0); err != nil {
					log.Printf("media status: %v", err)
				}
				if err := md.SendPlaybackMetadata("", "", "", nil, 0); err != nil {
					log.Printf("media metadata: %v", err)
				}
				cleared = true
				sentMeta = false
				return
			}
			cleared = false
			if err := md.SendPlaybackStatus(np.State, np.App, np.Position); err != nil {
				log.Printf("media status: %v", err)
			}
			next := session.PlaybackMeta{
				Title: np.Title, Artist: np.Artist, Album: np.Album,
				Duration: np.Duration, Art: np.ArtPNG,
			}
			if sentMeta && next.Same(meta) {
				return
			}
			if err := md.SendPlaybackMetadata(np.Title, np.Artist, np.Album, np.ArtPNG, np.Duration); err != nil {
				log.Printf("media metadata: %v", err)
				return
			}
			log.Printf("media metadata %q / %q / %q", np.Title, np.Artist, np.Album)
			meta = next
			sentMeta = true
		}
		publish()
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		for {
			select {
			case <-md.Done():
				return
			case <-tick.C:
				publish()
			}
		}
	}()
}
