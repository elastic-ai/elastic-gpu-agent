package types

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

type Device struct {
	Hash string
	List []string
}

func NewDevice(deviceList []string) *Device {
	curr := clone(deviceList)
	sort.Strings(curr)
	return &Device{
		Hash: hash(deviceList),
		List: curr,
	}
}

func (d *Device) Equals(o *Device) bool {
	return d.Hash == o.Hash && equals(d.List, o.List)
}

func clone(list []string) []string {
	ans := make([]string, len(list), len(list))
	copy(ans, list)
	return ans
}

func equals(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func hash(deviceList []string) string {
	to := func(bs [32]byte) []byte {
		return bs[0:32]
	}
	return hex.EncodeToString(to(sha256.Sum256([]byte(strings.Join(deviceList, ":")))))[0:8]
}