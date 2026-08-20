package shred

import (
	"crypto/rand"
	"os"
)

const DefaultPasses = 3

func ShredFile(path string, passes int) error {
	if passes < 1 {
		passes = DefaultPasses
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if info.IsDir() {
		return nil
	}

	size := info.Size()
	if size == 0 {
		return os.Remove(path)
	}

	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	buf := make([]byte, 4096)

	for pass := 0; pass < passes; pass++ {
		if _, err := f.Seek(0, 0); err != nil {
			return err
		}

		var fillByte byte
		switch pass % 3 {
		case 0:
			fillByte = 0x00
		case 1:
			fillByte = 0xFF
		case 2:
			for i := range buf {
				buf[i] = 0
			}
		}

		remaining := size
		for remaining > 0 {
			chunk := int64(len(buf))
			if remaining < chunk {
				chunk = remaining
			}

			if pass%3 == 2 {
				randBytes := make([]byte, chunk)
				if _, err := rand.Read(randBytes); err != nil {
					return err
				}
				if _, err := f.Write(randBytes[:chunk]); err != nil {
					return err
				}
			} else {
				for i := range int(chunk) {
					buf[i] = fillByte
				}
				if _, err := f.Write(buf[:chunk]); err != nil {
					return err
				}
			}
			remaining -= chunk
		}

		if err := f.Sync(); err != nil {
			return err
		}
	}

	if err := f.Close(); err != nil {
		return err
	}

	if err := os.Truncate(path, 0); err != nil {
		return err
	}

	return os.Remove(path)
}
