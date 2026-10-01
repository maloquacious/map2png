// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package map2png

import (
	"github.com/maloquacious/semver"
)

var (
	version = semver.Version{
		Major: 0,
		Minor: 3,
		Patch: 0,
	}
)

func Version() semver.Version {
	return version
}
