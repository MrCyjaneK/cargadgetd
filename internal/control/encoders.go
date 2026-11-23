package control

import "github.com/cargadgetd/cargadgetd/internal/videoenc"

func EncoderCount() int { return len(videoenc.Choices()) }

func EncoderAt(i int) (id, title string, ok bool) {
	all := videoenc.Choices()
	if i < 0 || i >= len(all) {
		return "", "", false
	}
	return all[i].ID, all[i].Title, true
}
