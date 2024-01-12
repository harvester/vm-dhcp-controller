package allocator

type Allocator interface {
	ListAll(string) (map[string]string, error)
}
