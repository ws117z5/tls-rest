// Package imagehash derives an image's public ref; shared by the images module and init/migrate_images_hash.sh.
package imagehash

import (
	"fmt"

	"github.com/ws117z5/mmh3"
)

const seed uint32 = 0x494d4147 // "IMAG"

// Of returns the MurmurHash3 (64-bit) of uuid as 16 hex characters.
func Of(uuid string) string {
	h, err := mmh3.Hash64(uuid, seed)
	if err != nil {
		return ""
	}
	sum := h.AsUint64()
	if len(sum) == 0 {
		return ""
	}
	return fmt.Sprintf("%016x", sum[0])
}
