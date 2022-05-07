package types

import (
	"crypto/sha256"
	"encoding/hex"
	v1 "k8s.io/api/core/v1"
	"sort"
	"strings"
)

type Device struct {
	Hash         string
	List         []string
	ResourceName v1.ResourceName
}

func NewDevice(deviceList []string, resourceName v1.ResourceName) *Device {
	curr := clone(deviceList)
	sort.Strings(curr)
	return &Device{
		Hash:         hash(curr),
		List:         curr,
		ResourceName: resourceName,
	}
}

func (d *Device) Equals(o *Device) bool {
	return d.Hash == o.Hash && equals(d.List, o.List) && d.ResourceName == o.ResourceName
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
