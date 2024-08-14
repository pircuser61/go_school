package sequence

type Config struct {
	WorkNumberPrefetchSize int `yaml:"work_number_prefetch_size"`
	PrefetchMinQueueSize   int `yaml:"prefetch_min_queue_size"`
}
