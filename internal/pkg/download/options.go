package download

import "sync"

var (
	Opt  *Options
	once sync.Once
)

func LoadOptions(cfg *Options) {
	once.Do(func() {
		Opt = cfg
	})
}
