package grocery

import "testing"

type Call interface{}

type FakeClient struct {
	t     *testing.T
	Calls chan Call
}

func NewFakeClient(t *testing.T) *FakeClient {
	return &FakeClient{t, make(chan Call)}
}

type allCall struct{}
type allResp struct {
	notes []*Note
	err   error
}

func (c *FakeClient) All() ([]*Note, error) {
	c.Calls <- &allCall{}
	resp := (<-c.Calls).(*allResp)
	return resp.notes, resp.err
}

type createCall struct{ note *Note }
type createResp struct{ err error }

func (c *FakeClient) Create(n *Note) error {
	c.Calls <- &createCall{n}
	return (<-c.Calls).(*createResp).err
}

func (c *FakeClient) Close() {
	close(c.Calls)
}

func (c *FakeClient) AssertCreate(n *Note, err error) {
	call := (<-c.Calls).(*createCall)
	if *call.note != *n {
		c.t.Error("expected create with", n, "but was", call.note)
	}
	c.Calls <- &createResp{err}
}

func (c *FakeClient) AssertAll(notes []*Note, err error) {
	call := (<-c.Calls).(*allCall)
	if call == nil {
		c.t.Error("No all call")
	}
	c.Calls <- &allResp{notes, err}
}

func (c *FakeClient) AssertDone(t *testing.T) {
	if _, more := <-c.Calls; more {
		t.Fatal("Did not expect more calls")
	}
}

func TestGroceryList(t *testing.T) {
	client := NewFakeClient(t)
	list := New()
	list.API = client

	go func() {
		client.AssertCreate(&Note{"apples"}, nil)
		client.AssertAll([]*Note{{"apples"}}, nil)
		client.Close()
	}()
	list.AddItem("apples")
	items, err := list.Items()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatal("expected one item")
	}
	if items[0] != "apples" {
		t.Fatal("expected apples")
	}

	client.AssertDone(t)
}
