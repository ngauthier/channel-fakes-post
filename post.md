# Creating Fakes in Go with Channels

Fakes are a common testing technique that involve creating a bare implementation of an interface that you can use in testing, and they usually allow you to check how they were used so you can ensure the behavior of the object under test. Normally, fakes have storage and retrieval methods, but in this post we're going to explore using Go's channels and goroutines to create incredible versatile fakes that also synchronize test code with executing code, even when the executing code is concurrent.

## The Unit

First, let's take a look at the code we're going to be testing. The object under test will be a Grocery List where you can add an item to the list and then retrieve all the items. The items will simply be strings. Additionally, the Grocery List will not be reposible for storing the items, it will have a Store that implements some API. Here's the code for the grocery list:

```go
type GroceryList struct {
	Store API
}

func New() *GroceryList {
	return &GroceryList{&HTTPClient{}}
}

func (g *GroceryList) AddItem(item string) error {
	return g.Store.Create(&Note{Text: item})
}

func (g *GroceryList) Items() ([]string, error) {
	notes, err := g.Store.All()
	if err != nil {
		return []string{}, err
	}

	items := make([]string, len(notes))
	for i := range notes {
		items[i] = notes[i].Text
	}

	return items, nil
}
```

Pretty simple, it's just interacting with the Store and using some sort of Note struct (this is from the API that we'll look at shortly). This often happens when you have to wrap some third party code, you make your own objects like Grocery List that have to conform to some other services data types and methods. But, this code should look pretty simple. Also note that when we use the constructor, we use some HTTPClient by default.

This is the object we'll be testing. But before we get into the test, let's take a look at the api we'll be using.

## The Collaborator

Since the unit under test is the Grocery List, any other code will be a collaborator, and we want to isolate Grocery List as best we can, so we'll use fakes for our collaborators. But first, let's look at the API and Note and HTTPClient:

```go
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
```

API simply has a Create and All method that operate on these Notes, and Notes just wrap some text. We also have a Create and All method on the HTTPClient. Imagine, for a moment, that this code was given to you by another department or another company or service or whatever. We don't have the freedom to change this code, and we want to isolate its implementation as best we can when we go to test the Grocery List. To do that, we'll make a FakeClient that implements API and inject it into our Grocery List so it doesn't matter that we don't even have an implementation right now. And actually, that's really nice, because you can write your higher order objects first, then come back around and implement the low level clients later.

## Testing Part One: The Draft

Before we implement our fake, let's look at the behavior we want to test. This first test is just a draft, so we don't expect it to pass or be the real test, but it will help us get our head in the right place:

```go
func TestGroceryList(t *testing.T) {
	list := New()
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
}
```

So, what we're going to do is create a new Grocery List, add "apples" to it, and make sure we get "apples" back. If we used a real working Store, this test would pass. But we don't want to use a working Store. Maybe because setting up a Store http server to interact with is a big pain, or maybe we're just great programmers that want to isolate our code :).

## Injecting a Fake and Making an Assertion

Let's start by injecting a fake and doing assertions in the background. We'll start with one assertion and implement it before doing the second one. First, here's our test with one assertion:

```go
func TestGroceryList(t *testing.T) {
	client := NewFakeClient(t)
	list := New()
	list.Store = client

	go func() {
		client.AssertCreate(&Note{"apples"}, nil)
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
```

So, we create a FakeClient and give it the testing instance so it can make assertions. Then we inject it as the Grocery List's store. This is called Setter Injection. Next, in a goroutine we are going to call AssertCreate and pass in what we expect the Store's Create method to be called with: a Note with "apples". We also pass the return value to send back to the Grocery List: a nil error. Then we Close the client. Also note at the end of the test we call AssertDone. AssertDone is just going to wait for the Close so that our goroutine doesn't get lost.

So, AssertCreate is pretty cool, because we get to say what to return and also what we expect it to be called with. That means we can use this over and over, and we don't have to store lots of call data on the fake object. We just call the method each time.

## Channel Time

OK, let's dive into the implementation of the FakeClient:

```go
type Call interface{}

type FakeClient struct {
        t     *testing.T
        Calls chan Call
}

func NewFakeClient(t *testing.T) *FakeClient {
        return &FakeClient{t, make(chan Call)}
}

type createCall struct{ note *Note }
type createResp struct{ err error }

func (c *FakeClient) Create(n *Note) error {
        c.Calls <- &createCall{n}
        return (<-c.Calls).(*createResp).err
}

func (c *FakeClient) AssertCreate(n *Note, err error) {
        call := (<-c.Calls).(*createCall)
        if *call.note != *n {
                c.t.Error("expected create with", n, "but was", call.note)
        }
        c.Calls <- &createResp{err}
}

func (c *FakeClient) Close() {
        close(c.Calls)
}

func (c *FakeClient) AssertDone(t *testing.T) {
	if _, more := <-c.Calls; more {
		t.Fatal("Did not expect more calls")
	}
}
```

OK, let's walk through this. A FakeClient has a testing instance and a Calls channel. Calls are just `interface{}` objects, so we can send anything here. The NewFakeClient constructor is straightforward. Next we have `createCall` and `createResp` objects. These structs hold the paramaters for Create and the return value. Their fields should match the params and return values exactly.

Now look at Create. What we do is we send a createCall on the calls channel with the param. Then we receive off the calls channel a createResp, which we return as the return value. So this Create can receive and respond with anything we want! We don't have to actually store and pop calls, we'll use channels! Cool!

AssertCreate takes both the param and return value, and what it does is it receives the createCall from the client, then performs the assertion that the parameter we expect is what was called. In this case we want to make sure the Note values are equal. Then, we send a createResp holding the error value back to the fake client.

Lastly, we have Close which just closes the channel, and AssertDone which makes sure there was nothing left on the channel.


Scroll back up and look at the test. We are running the assertions in one routing, and the client in the other. This way they actually synchronize between each other, so when a method is called on the client, we MUST have an assert call, or we'll get a deadlock. This is actually a really cool feature, because it means all calls must be accounted for in the exact order they're called.

Finally, we also assert that we didn't miss any calls afterwards.

## Call and Response

1. Go through the same thing with AssertAll
1. Note the how the call and resp objects change and func signature changes
1. Conclusion, mention go generate
