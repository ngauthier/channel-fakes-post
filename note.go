package grocery

type API interface {
	Create(*Note) error
	All() ([]*Note, error)
}

type Note struct {
	Text string
}

type HTTPClient struct {
}

func (c *HTTPClient) Create(n *Note) error {
	// some implementation

	return nil
}

func (c *HTTPClient) All() ([]*Note, error) {
	// some implementation

	return []*Note{}, nil
}
