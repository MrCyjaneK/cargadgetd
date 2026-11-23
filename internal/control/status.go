package control

import "os"

func FFSReady() bool {
	_, err := os.Stat(FFSEp)
	return err == nil
}
