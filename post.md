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

## Injecting a Fake and Making an Assertion

Let's start by injecting a fake and doing assertions in the background. We'll start with one method and implement it before doing the second one. First, here's our test:

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
	client.AssertDone(t)
}
```

So, we create a FakeClient and give it the testing instance so it can make assertions. Then we inject it as the Grocery List's store. This is called Setter Injection. Next, in a goroutine we are going to call AssertCreate and pass in what we expect the Store's Create method to be called with: a Note with "apples". We also pass the return value to send back to the Grocery List: a nil error. Then we Close the client. Also note at the end of the test we call AssertDone. AssertDone is just going to wait for the Close so that our goroutine doesn't get lost, and it will also make sure there aren't any extra calls.

So, AssertCreate is pretty cool, because we get to say what we expect it to be called with plus what to return to the caller. That means we can use this over and over, and we don't have to store lots of call data on the fake object. We just call the method each time.

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


Scroll back up and look at the test. We are running the assertions in one routine, and the client in the other. This way they actually synchronize between each other, so when a method is called on the client, we MUST have an assert call, or we'll get a deadlock. This is actually a really cool feature, because it means all calls must be accounted for in the exact order they're called. At the end of the test, we also assert that we didn't miss any calls afterwards.

## Repeat

OK, now that we understand how to create a Grocery List item and verify its creation, let's run through the same process but for our other method, Items(). To review, let's look at the Items method on Grocery List:

```go
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

To test Items we need to write a test to cover it:

```go
func TestGroceryListAll(t *testing.T) {
	client := NewFakeClient(t)
	list := New()
	list.Store = client

	go func() {
		client.AssertAll([]*Note{{"apples"}}, nil)
		client.Close()
	}()
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

OK, this one's a little longer. We do the usual setup at the beginning, and we have the same assert call and close in our goroutine. But we've added a little more assertions on the outside. We call Items, but then we also check to make sure the items we get back are an array containing the one string "apples". This way we can make sure that Items properly parses the Note objects into strings and returns them.

At this point, making the fake's implementation is exactly the same as the previous one for Create, except with a different signature and structs for the params and return values:

```go
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

func (c *FakeClient) AssertAll(notes []*Note, err error) {
	call := (<-c.Calls).(*allCall)
	if call == nil {
		c.t.Error("No all call")
	}
	c.Calls <- &allResp{notes, err}
}
```

We have our Call and Resp structs matching the params and return values as usual. The All call is almost the same except we need to assign the response to a variable so we can access the multiple return values. Then in our assert there are no params to assert so we can just make sure the call is not nil and return the desired return values on the channel.

And that's about it! Our tests should pass.

## Conclusion

So, in conclusion, we've created a simple repeatable pattern that builds fakes for any interface that can be used in many different combinations without modification. That means once we write the fake method, we can write any tests we want without modifying the fake. It's really nice and flexible. Plus, the synchronization from using unbuffered channels means that our implementation and test are lined right up.

One thing I'd really like to improve on is the generation of the fakes. Since they depend just on the interface, I'd love to write a go generate library that could build fakes for me. This will be something I'll work on in the future I'm sure.

Finally, as a quick tip, I found the Ctrl+\ key combination invaluable once I have a couple of fakes and background routines running. When you use timeouts and ticks sometimes you can be in a deadlock but not actually cause a deadlock (because you're looping on a select, for example). Using Ctrl+\ you can force a test to break and output a backtrace for every goroutine. If I didn't have Ctrl+\ I would have gone nuts!
