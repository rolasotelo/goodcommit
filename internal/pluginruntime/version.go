package pluginruntime

import (
	"runtime/debug"
	"strings"
	"sync"
)

var (
	builtinRefOnce sync.Once
	builtinRef     string
)

func builtinSourceRef() string {
	builtinRefOnce.Do(func() {
		info, ok := debug.ReadBuildInfo()
		if !ok {
			return
		}
		v := strings.TrimSpace(info.Main.Version)
		if v == "" || v == "(devel)" {
			for _, setting := range info.Settings {
				if setting.Key == "vcs.revision" {
					ref := strings.TrimSpace(setting.Value)
					if ref != "" {
						builtinRef = ref
						return
					}
				}
			}
			return
		}
		builtinRef = v
	})
	return builtinRef
}
